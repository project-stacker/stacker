package mount

import (
	"bufio"
	"os"
	"strings"

	"github.com/pkg/errors"
)

type Mount struct {
	Source string
	Target string
	FSType string
	Opts   []string
}

func (m Mount) GetOverlayDirs() ([]string, error) {
	if m.FSType != "overlay" {
		return nil, errors.Errorf("%s is not an overlayfs", m.Target)
	}

	for _, opt := range m.Opts {
		if !strings.HasPrefix(opt, "lowerdir=") {
			continue
		}

		return strings.Split(strings.TrimPrefix(opt, "lowerdir="), ":"), nil
	}

	return nil, errors.Errorf("no lowerdirs found")
}

type Mounts []Mount

func (ms Mounts) FindMount(p string) (Mount, bool) {
	for _, m := range ms {
		if m.Target == p {
			return m, true
		}
	}

	return Mount{}, false
}

func ParseMounts(mountinfo string) (Mounts, error) {
	f, err := os.Open(mountinfo)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't open %s", mountinfo)
	}
	defer f.Close()

	mounts := []Mount{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		mount := Mount{}
		mount.Target = fields[4]

		for i := 5; i < len(fields); i++ {
			if fields[i] != "-" {
				continue
			}

			mount.FSType = fields[i+1]
			mount.Source = fields[i+2]
			mount.Opts = strings.Split(fields[i+3], ",")
		}

		mounts = append(mounts, mount)
	}

	return mounts, nil
}

func IsMountpoint(target string) (bool, error) {
	_, mounted, err := FindMount(target)
	return mounted, err
}

func FindMount(target string) (Mount, bool, error) {
	mounts, err := ParseMounts("/proc/self/mountinfo")
	if err != nil {
		return Mount{}, false, err
	}

	for _, mount := range mounts {
		if mount.Target == strings.TrimRight(target, "/") {
			return mount, true, nil
		}
	}

	return Mount{}, false, nil
}
