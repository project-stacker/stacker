package stacker

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/lxc/go-lxc.v2"
)

const ReasonableDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin/bin"

// our representation of a container
type container struct {
	sc StackerConfig
	c  *lxc.Container
}

func newContainer(sc StackerConfig, name string) (*container, error) {
	if !lxc.VersionAtLeast(2, 1, 0) {
		return nil, fmt.Errorf("stacker requires liblxc >= 2.1.0")
	}

	lxcC, err := lxc.NewContainer(name, sc.RootFSDir)
	if err != nil {
		return nil, err
	}
	c := &container{sc: sc, c: lxcC}

	err = os.MkdirAll(path.Join(sc.StackerDir, "logs"), 0755)
	if err != nil {
		return nil, err
	}

	err = c.c.SetLogFile(path.Join(sc.StackerDir, "logs", name))
	if err != nil {
		return nil, err
	}

	err = c.c.SetLogLevel(lxc.TRACE)
	if err != nil {
		return nil, err
	}

	configs := map[string]string{
		// ->execute() seems to set these up for is; if we provide
		// them, we get an EBUSY for sysfs
		//"lxc.mount.auto": "proc:mixed sys:mixed cgroup:mixed",
		"lxc.autodev":     "1",
		"lxc.uts.name":    name,
		"lxc.net.0.type":  "none",
		"lxc.environment": fmt.Sprintf("PATH=%s", ReasonableDefaultPath),
	}

	if err := c.setConfigs(configs); err != nil {
		return nil, err
	}

	rootfs := path.Join(sc.RootFSDir, name, "rootfs")
	err = c.setConfig("lxc.rootfs.path", fmt.Sprintf("dir:%s", rootfs))
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

func (c *container) execute(args string) error {
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

	cmd := exec.Command(
		os.Args[0],
		"internal",
		c.c.Name(),
		c.sc.RootFSDir,
		f.Name(),
	)

	// These should all be non-interactive; let's ensure that.
	cmd.Stdin = nil

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run commands failed: %s", err)
	}

	return nil
}
