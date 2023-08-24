package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lxc/go-lxc"
	"github.com/pkg/errors"
	embed_exec "stackerbuild.io/stacker/pkg/embed-exec"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

const (
	ReasonableDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	lxcConfigRootfsPath   = "lxc.rootfs.path"
)

// our representation of a container
type Container struct {
	sc      types.StackerConfig
	c       *lxc.Container
	workdir string
}

func New(sc types.StackerConfig, name string) (*Container, error) {
	if !lxc.VersionAtLeast(2, 1, 0) {
		return nil, errors.Errorf("stacker requires liblxc >= 2.1.0")
	}

	if err := os.MkdirAll(sc.RootFSDir, 0755); err != nil {
		return nil, errors.WithStack(err)
	}

	workdir, err := os.MkdirTemp("", "c*")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	lxcC, err := lxc.NewContainer(name, sc.RootFSDir)
	if err != nil {
		return nil, err
	}
	c := &Container{sc: sc, c: lxcC, workdir: workdir}

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

	return c, nil
}

func (c *Container) BindMount(source string, dest string, extraOpts string) error {
	createOpt := "create=dir"
	stat, err := os.Stat(source)
	if err == nil && !stat.IsDir() {
		createOpt = "create=file"
	}

	val := fmt.Sprintf("%s %s none rbind,%s,%s 0 0", source, strings.TrimPrefix(dest, "/"), createOpt, extraOpts)
	return c.SetConfig("lxc.mount.entry", val)
}

func (c *Container) SetConfigs(config map[string]string) error {
	for k, v := range config {
		if err := c.SetConfig(k, v); err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) SetConfig(name string, value string) error {
	var err error
	if name == lxcConfigRootfsPath {
		err = c.setRootfs(value)
	} else {
		err = c.c.SetConfigItem(name, value)
	}
	if err != nil {
		return errors.Errorf("failed setting config %s to %s: %v", name, value, err)
	}
	return nil
}

func (c *Container) setRootfs(rootfs string) error {
	newRootfs, err := shortenRootfs(rootfs, c.workdir)
	if err != nil {
		return err
	}
	return c.c.SetConfigItem(lxcConfigRootfsPath, newRootfs)
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

func (c *Container) Execute(args []string, stdin io.Reader) error {
	f, err := os.CreateTemp("", fmt.Sprintf("stacker_%s_run", c.c.Name()))
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := c.c.SaveConfigFile(f.Name()); err != nil {
		return errors.WithStack(err)
	}

	// we want to be sure to remove the /stacker from the generated
	// filesystem after execution. we should probably parameterize this in
	// the storage API.
	defer os.RemoveAll(path.Join(c.sc.RootFSDir, c.c.Name(), "overlay", "stacker"))

	cmd, cleanup, err := embed_exec.GetCommand(
		c.sc.EmbeddedFS,
		"lxc-wrapper/lxc-wrapper",
		append([]string{"spawn", c.c.Name(), c.sc.RootFSDir, f.Name()}, args...)...,
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

func (c *Container) SaveConfigFile(p string) error {
	return c.c.SaveConfigFile(p)
}

func (c *Container) Close() {
	c.c.Release()
	if c.workdir != "" {
		os.RemoveAll(c.workdir)
	}
}

// shortenRootfs - shorten an lxc rootfs string to protect from PATH_MAX.
// mount paths are limited, but they can be shortened by using a symlink trick.
// a rootfs value for an overlayfs is of the form:
//
//	overlay:overlayfs:<roots>/<name>/<path1>:<roots>/<name>/<path2>...,<options>
//
// path1 and path2 above are often 'sha256_<hash>/overlay' (which has string length 82)
//
// shortenRootfs creates symlinks in workdir named '00', '01'...
//
//	<workdir>/00 -> <stacker-roots>/sha256_.../overlay
//	<workdir>/01 -> <stacker-roots>/sha267_.../overlay
//
// so instead of paths with length (len(rootsdir) + 80)
// you end up with paths of len(workdir) + 2, or workdir + 3 if there are > 99 paths.
//
// The use case where we became aware of this had a rootsdir with path lenth 82,
// and 47 path elements. Its total length was 7580. If shortened with a workdir
// /tmp/c789283801/ (len 16), then the end result is 910 chars.
func shortenRootfs(rootfs string, workdir string) (string, error) {
	// We could simply return if len(rootfs) was < 4096.
	// That would avoid the workdir indirection in almost all cases, but
	// would also mean that this code is not tested in almost all cases.
	// Better to have it tested than be basically dead code.
	//
	// if len(rootfs) < 4096 {
	// 	return rootfs, nil
	// }

	const prefix = "overlay:overlayfs:"
	sansPrefix := strings.TrimPrefix(rootfs, prefix)
	if sansPrefix == rootfs {
		return rootfs, nil
	}

	options := ""
	toks := strings.SplitN(sansPrefix, ",", 2)
	if len(toks) > 1 {
		options = "," + toks[1]
	}

	paths := strings.Split(toks[0], ":")
	dfmt := "%02d"
	if len(paths) > 999 {
		return "", errors.Errorf("too many paths (%d) in rootfs string: %s", len(paths), rootfs)
	} else if len(paths) > 99 {
		dfmt = "%03d"
	}

	newPaths := []string{}
	for pnum, opath := range paths {
		if !filepath.IsAbs(opath) {
			return "", errors.Errorf("path %d is not absolute: %s", pnum, opath)
		}
		newPath := filepath.Join(workdir, fmt.Sprintf(dfmt, pnum))
		if err := os.Symlink(opath, newPath); err != nil {
			return "", errors.Wrapf(err, "failed to symlink %s -> %s", newPath, opath)
		}
		newPaths = append(newPaths, newPath)
	}

	newrootfs := prefix + strings.Join(newPaths, ":") + options
	if len(newrootfs) > 4096 {
		return "", errors.Errorf(
			"overlayfs rootfs value with %d layers is > 4096 (%d),"+
				" shortened version via '%s' was still too long (%d)",
			len(paths), len(rootfs), workdir, len(newrootfs))
	}
	log.Debugf("Shortened overlayfs rootfs value with %d layers via workdir %s from %d to %d",
		len(paths), workdir, len(rootfs), len(newrootfs))
	return newrootfs, nil
}
