package mount

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
)

func filesystemIsSupported(fs string, procFilesystems io.Reader) (bool, error) {
	scanner := bufio.NewScanner(procFilesystems)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		filesystem := fields[len(fields)-1]

		if filesystem == fs {
			return true, nil
		}
	}

	return false, nil
}

func FilesystemIsSupported(fs string) (bool, error) {
	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return false, errors.WithStack(err)
	}
	defer f.Close()

	return filesystemIsSupported(fs, f)
}
