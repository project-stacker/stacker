package stacker

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"

	"github.com/lxc/lxd/shared/idmap"
	"github.com/openSUSE/umoci/oci/layer"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"gopkg.in/lxc/go-lxc.v2"
)

const (
	ReasonableDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

var (
	IdmapSet *idmap.IdmapSet
)

func init() {
	if os.Geteuid() != 0 {
		currentUser, err := user.Current()
		if err != nil {
			return
		}

		// An error here means that this user has no subuid
		// delegations. The only thing we can do is panic, and if we're
		// re-execing inside a user namespace we don't want to do that.
		// So let's just ignore the error and let future code handle it.
		IdmapSet, _ = idmap.DefaultIdmapSet(currentUser.Username)
	}
}

// HostIDInUserns returns the uid that the host uid of stacker will be mapped
// to when calling RunInUserns.
func HostIDInUserns() (int64, error) {
	if IdmapSet == nil {
		return -1, fmt.Errorf("no idmap")
	}

	max := int64(100000)
	for _, idm := range IdmapSet.Idmap {
		if idm.Nsid+idm.Maprange >= max {
			max = idm.Nsid + idm.Maprange + 1
		}
	}

	return max, nil
}

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

	if err := c.c.SetLogLevel(lxc.TRACE); err != nil {
		return nil, err
	}

	err = c.c.SetLogFile(path.Join(sc.StackerDir, "logs", name))
	if err != nil {
		return nil, err
	}

	if IdmapSet != nil {
		for _, idm := range IdmapSet.Idmap {
			if err := idm.Usable(); err != nil {
				return nil, fmt.Errorf("idmap unusable: %s", err)
			}
		}

		for _, lxcConfig := range IdmapSet.ToLxcString() {
			err = c.setConfig("lxc.id_map", lxcConfig)
			if err != nil {
				return nil, err
			}
		}

		// If we're in a userns, we need to be sure and make sure the
		// rootfs pivot dir is somewhere that we can actually write to.
		// Let's use .stacker/rootfs instead of /var/lib/lxc/rootfs
		rootfsPivot := path.Join(sc.StackerDir, "rootfsPivot")
		if err := os.MkdirAll(rootfsPivot, 0755); err != nil {
			return nil, err
		}

		if err := c.setConfig("lxc.rootfs.mount", rootfsPivot); err != nil {
			return nil, err
		}
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

func umociMapOptions() *layer.MapOptions {
	os := &layer.MapOptions{}
	if IdmapSet == nil {
		return os
	}

	os.UIDMappings = []rspec.LinuxIDMapping{}
	os.GIDMappings = []rspec.LinuxIDMapping{}
	os.Rootless = true

	for _, ide := range IdmapSet.Idmap {
		if ide.Isuid {
			os.UIDMappings = append(os.UIDMappings, rspec.LinuxIDMapping{
				HostID:      uint32(ide.Hostid),
				ContainerID: uint32(ide.Nsid),
				Size:        uint32(ide.Maprange),
			})
		}

		if ide.Isgid {
			os.GIDMappings = append(os.GIDMappings, rspec.LinuxIDMapping{
				HostID:      uint32(ide.Hostid),
				ContainerID: uint32(ide.Nsid),
				Size:        uint32(ide.Maprange),
			})
		}
	}

	return os
}

func RunInUserns(userCmd []string, msg string) error {
	id, err := HostIDInUserns()
	if err != nil {
		return err
	}

	args := []string{
		"-m",
		fmt.Sprintf("b:%d:%d:1", id, os.Getuid()),
	}

	for _, idm := range IdmapSet.Idmap {
		var which string
		if idm.Isuid && idm.Isgid {
			which = "b"
		} else if idm.Isuid {
			which = "u"
		} else if idm.Isgid {
			which = "g"
		}

		m := fmt.Sprintf("%s:%d:%d:%d", which, idm.Nsid, idm.Hostid, idm.Maprange)
		args = append(args, "-m", m)
	}

	args = append(args, "--")
	args = append(args, userCmd...)
	cmd := exec.Command("lxc-usernsexec", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error %s: %s", msg, err)
	}

	return nil
}
