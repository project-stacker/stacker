package stacker

import (
	"bufio"
	"path"
	"os"
	"fmt"
	"strings"
	"time"

	"gopkg.in/lxc/go-lxc.v2"
	"github.com/pkg/errors"
)

// our representation of a container
type container struct {
	sc StackerConfig
	c *lxc.Container
}

func newContainer(sc StackerConfig, name string) (*container, error) {
	lxcC, err := lxc.NewContainer(name, sc.RootFSDir)
	if err != nil {
		return nil, err
	}
	c := &container{sc: sc, c: lxcC}

	configs := map[string]string{
		// ->execute() seems to set these up for is; if we provide
		// them, we get an EBUSY for sysfs
		//"lxc.mount.auto": "proc:mixed sys:mixed cgroup:mixed",
		"lxc.autodev": "1",
		"lxc.uts.name": name,
		"lxc.net.0.type": "none",
	}

	if err := c.c.SetLogLevel(lxc.TRACE); err != nil {
		return nil, err
	}

	if err := c.setConfigs(configs); err != nil {
		return nil, err
	}

	rootfs := path.Join(sc.RootFSDir, ".working")
	if lxc.VersionAtLeast(2, 1, 0) {
		err := c.setConfig("lxc.rootfs.path", fmt.Sprintf("dir:%s", rootfs))
		if err != nil {
			return nil, err
		}
	} else {
		// stacker is explicitly managing things
		if err := c.setConfig("lxc.rootfs.backend", "dir"); err != nil {
			return nil, err
		}

		if err := c.setConfig("lxc.rootfs", rootfs); err != nil {
			return nil, err
		}
	}


	err = os.MkdirAll(path.Join(sc.StackerDir, "logs"), 0755)
	if err != nil {
		return nil, err
	}

	err = c.c.SetLogFile(path.Join(sc.StackerDir, "logs", name))
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *container) bindMount(source string, dest string) error {
	createOpt := "create=dir"
	stat, err := os.Lstat(source)
	if err == nil && !stat.IsDir() {
		createOpt = "create=file"
	}

	val := fmt.Sprintf("%s %s none rbind,%s", source, strings.TrimPrefix(dest, "/"), createOpt)
	return c.setConfig("lxc.mount.entry", val)
}

func (c *container) setConfigs(config map[string]string) error {
	for k, v := range config {
		if err := c.setConfig(k, v); err != nil {
			return err
		}
	}

	return nil
}

func (c *container) setConfig(name string, value string) error {
	err := c.c.SetConfigItem(name, value)
	if err != nil {
		return fmt.Errorf("failed setting config %s to %s: %v", name, value, err)
	}
	return nil
}

func (c *container) logPath() string {
	return path.Join(c.sc.StackerDir, "logs", c.c.Name())
}

// containerError tries its best to report as much context about an LXC error
// as possible.
func (c *container) containerError(theErr error, msg string) error {
	if theErr == nil {
		return nil
	}

	f, err := os.Open(c.logPath())
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
		lxcErrors = append(lxcErrors, line)
	}

	extra := strings.Join(lxcErrors[len(lxcErrors)-10:], "\n")
	return errors.Wrap(theErr, fmt.Sprintf("%s\nLast few LXC errors:\n%s\n", msg, extra))
}

func (c *container) execute(args []string) error {
	err := c.c.StartExecute(args)
	if err != nil {
		return c.containerError(err, "execute failed")
	}

	// If the command exits too fast, this will return false, since there
	// was nothing to wait for. so let's explicitly ignore the return code.
	c.c.Wait(lxc.STOPPED, -1 * time.Second)
	return nil
}
