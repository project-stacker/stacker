package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
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
)

// our representation of a container
type Container struct {
	sc types.StackerConfig
	c  *lxc.Container
}

func New(sc types.StackerConfig, name string) (*Container, error) {
	if !lxc.VersionAtLeast(2, 1, 0) {
		return nil, errors.Errorf("stacker requires liblxc >= 2.1.0")
	}

	if err := os.MkdirAll(sc.RootFSDir, 0755); err != nil {
		return nil, errors.WithStack(err)
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
	if err := c.SetConfig("lxc.execute.cmd", args); err != nil {
		return err
	}

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
	defer os.Remove(path.Join(c.sc.RootFSDir, c.c.Name(), "overlay", "stacker"))

	cmd, cleanup, err := embed_exec.GetCommand(
		c.sc.EmbeddedFS,
		"lxc-wrapper/lxc-wrapper",
		"spawn",
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

func (c *Container) SaveConfigFile(p string) error {
	return c.c.SaveConfigFile(p)
}

func (c *Container) Close() {
	c.c.Release()
}
