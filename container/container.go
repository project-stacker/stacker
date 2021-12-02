package container

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	stackeridmap "github.com/anuvu/stacker/container/idmap"
	embed_exec "github.com/anuvu/stacker/embed-exec"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/types"
	"github.com/lxc/go-lxc"
	"github.com/pkg/errors"
)

const (
	ReasonableDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

func runInternalGoSubcommand(config types.StackerConfig, args []string) error {
	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	cmd := []string{
		"--oci-dir", config.OCIDir,
		"--roots-dir", config.RootFSDir,
		"--stacker-dir", config.StackerDir,
		"--storage-type", config.StorageType,
		"--internal-userns",
	}

	if config.Debug {
		cmd = append(cmd, "--debug")
	}

	cmd = append(cmd, "internal-go")
	cmd = append(cmd, args...)
	c := exec.Command(binary, cmd...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return errors.WithStack(c.Run())
}

// our representation of a container
type Container struct {
	sc types.StackerConfig
	c  *lxc.Container
}

func New(sc types.StackerConfig, storage types.Storage, name string) (*Container, error) {
	if !lxc.VersionAtLeast(2, 1, 0) {
		return nil, errors.Errorf("stacker requires liblxc >= 2.1.0")
	}

	lxcC, err := lxc.NewContainer(name, sc.RootFSDir)
	if err != nil {
		return nil, err
	}
	c := &Container{sc: sc, c: lxcC}

	if err := c.c.SetLogLevel(lxc.TRACE); err != nil {
		return nil, err
	}

	logFile := path.Join(sc.StackerDir, "lxc.log")
	err = c.c.SetLogFile(logFile)
	if err != nil {
		return nil, err
	}

	// Truncate the log file by hand, so people don't get confused by
	// previous runs.
	err = os.Truncate(logFile, 0)
	if err != nil {
		return nil, err
	}

	idmapSet, err := stackeridmap.ResolveCurrentIdmapSet()
	if err != nil {
		return nil, err
	}

	// similar to the hard coding in MaybeRunInUserns(), for now root
	// containers run as root.
	if os.Geteuid() == 0 {
		idmapSet = nil
	}

	if idmapSet != nil {
		for _, idm := range idmapSet.Idmap {
			if err := idm.Usable(); err != nil {
				return nil, errors.Errorf("idmap unusable: %s", err)
			}
		}

		for _, lxcConfig := range idmapSet.ToLxcString() {
			err = c.setConfig("lxc.idmap", lxcConfig)
			if err != nil {
				return nil, err
			}
		}

	}

	rootfsPivot := path.Join(sc.StackerDir, "rootfsPivot")
	if err := os.MkdirAll(rootfsPivot, 0755); err != nil {
		return nil, err
	}

	if err := c.setConfig("lxc.rootfs.mount", rootfsPivot); err != nil {
		return nil, err
	}

	configs := map[string]string{
		"lxc.mount.auto":                "proc:mixed",
		"lxc.autodev":                   "1",
		"lxc.pty.max":                   "1024",
		"lxc.mount.entry":               "none dev/shm tmpfs defaults,create=dir 0 0",
		"lxc.uts.name":                  name,
		"lxc.net.0.type":                "none",
		"lxc.environment":               fmt.Sprintf("PATH=%s", ReasonableDefaultPath),
		"lxc.apparmor.allow_incomplete": "1",
	}

	if err := c.setConfigs(configs); err != nil {
		return nil, err
	}

	err = c.BindMount("/sys", "/sys", "")
	if err != nil {
		return nil, err
	}

	err = c.BindMount("/etc/resolv.conf", "/etc/resolv.conf", "")
	if err != nil {
		return nil, err
	}

	rootfs, err := storage.GetLXCRootfsConfig(name)
	if err != nil {
		return nil, err
	}

	err = c.setConfig("lxc.rootfs.path", rootfs)
	if err != nil {
		return nil, err
	}

	// liblxc inserts an apparmor profile if we don't set one by default.
	// however, since we may be statically linked with no packaging
	// support, the host may not have this default profile. let's check for
	// it. of course, we can't check for it by catting the value in
	// securityfs, because that's restricted :). so we fork and try to
	// change to the profile in question instead.
	//
	// note that this is not strictly correct: lxc will try to use a
	// non-cgns profile if cgns isn't supported by the kernel, but most
	// kernels these days support it so we ignore this case.
	lxcDefaultProfile := "lxc-container-default-cgns"
	err = runInternalGoSubcommand(sc, []string{"check-aa-profile", lxcDefaultProfile})
	if err != nil {
		log.Infof("couldn't find AppArmor profile %s", lxcDefaultProfile)
		err = c.setConfig("lxc.apparmor.profile", "unconfined")
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Container) BindMount(source string, dest string, extraOpts string) error {
	createOpt := "create=dir"
	stat, err := os.Stat(source)
	if err == nil && !stat.IsDir() {
		createOpt = "create=file"
	}

	val := fmt.Sprintf("%s %s none rbind,%s,%s 0 0", source, strings.TrimPrefix(dest, "/"), createOpt, extraOpts)
	return c.setConfig("lxc.mount.entry", val)
}

func (c *Container) setConfigs(config map[string]string) error {
	for k, v := range config {
		if err := c.setConfig(k, v); err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) setConfig(name string, value string) error {
	err := c.c.SetConfigItem(name, value)
	if err != nil {
		return errors.Errorf("failed setting config %s to %s: %v", name, value, err)
	}
	return nil
}

// containerError tries its best to report as much context about an LXC error
// as possible.
func (c *Container) containerError(theErr error, msg string) error {
	if theErr == nil {
		return nil
	}

	f, err := os.Open(c.c.LogFile())
	if err != nil {
		return errors.Wrap(theErr, msg)
	}

	lxcErrors := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "ERROR") {
			lxcErrors = append(lxcErrors, line)
		}
	}

	for _, err := range lxcErrors {
		log.Debugf(err)
	}
	return errors.Wrapf(theErr, msg)
}

func (c *Container) Execute(args string, stdin io.Reader) error {
	if err := c.setConfig("lxc.execute.cmd", args); err != nil {
		return err
	}

	f, err := ioutil.TempFile("", fmt.Sprintf("stacker_%s_run", c.c.Name()))
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := c.c.SaveConfigFile(f.Name()); err != nil {
		return err
	}

	// we want to be sure to remove the /stacker from the generated
	// filesystem after execution. TODO: parameterize this by storage
	// backend? it will always be "rootfs" for btrfs and "overlay" for the
	// overlay backend. Maybe this shouldn't even live here.
	defer os.Remove(path.Join(c.sc.RootFSDir, c.c.Name(), "rootfs", "stacker"))
	defer os.Remove(path.Join(c.sc.RootFSDir, c.c.Name(), "overlay", "stacker"))

	cmd, cleanup, err := embed_exec.GetCommand(
		c.sc.EmbeddedFS,
		"lxc-wrapper/lxc-wrapper",
		c.c.Name(),
		c.sc.RootFSDir,
		f.Name(),
	)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// If this is non-interactive, we're going to setsid() later, so we
	// need to make sure we capture the output somehow.
	if stdin == nil {
		reader, writer := io.Pipe()
		defer writer.Close()

		cmd.Stdout = writer
		cmd.Stderr = writer

		go func() {
			defer reader.Close()
			_, err := io.Copy(os.Stdout, reader)
			if err != nil {
				log.Infof("err from stdout copy: %s", err)
			}
		}()

	}

	signals := make(chan os.Signal, 32)
	signal.Notify(signals)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case sg := <-signals:
				// ignore SIGCHLD, we can't forward it and it's
				// meaningless anyway
				if sg == syscall.SIGCHLD {
					continue
				}

				// upgrade SIGINT to SIGKILL. bash exits when
				// it receives SIGINT, but doesn't kill its
				// children, meaning the currently executing
				// command will keep executing until it
				// completes, and *then* things will die.
				// Instead, let's just force kill it.
				if sg == syscall.SIGINT {
					sg = syscall.SIGKILL
				}

				err = syscall.Kill(c.c.InitPid(), sg.(syscall.Signal))
				if err != nil {
					log.Infof("failed to send signal %v %v", sg, err)
				}
			}
		}
	}()

	cmdErr := cmd.Run()
	done <- true

	return c.containerError(cmdErr, "execute failed")
}

func (c *Container) SetupLayerConfig(l *types.Layer, name string) error {
	env, err := l.BuildEnvironment(name)
	if err != nil {
		return err
	}

	importsDir := path.Join(c.sc.StackerDir, "imports", c.c.Name())
	if _, err := os.Stat(importsDir); err == nil {
		log.Debugf("bind mounting %s into container", importsDir)
		err = c.BindMount(importsDir, "/stacker", "ro")
		if err != nil {
			return err
		}
	} else {
		log.Debugf("not bind mounting %s into container", importsDir)
	}

	for k, v := range env {
		if v != "" {
			err = c.setConfig("lxc.environment", fmt.Sprintf("%s=%s", k, v))
			if err != nil {
				return err
			}
		}
	}

	binds, err := l.ParseBinds()
	if err != nil {
		return err
	}

	for source, target := range binds {
		err = c.BindMount(source, target, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) SaveConfigFile(p string) error {
	return c.c.SaveConfigFile(p)
}

func (c *Container) Close() {
	c.c.Release()
}
