// This is a package to go from an embed.FS + file name to an exec.Command;
// works only on recent linux kernels
package embed_exec

import (
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/justincormack/go-memfd"
	"github.com/pkg/errors"
)

func GetCommand(fs embed.FS, filename string, args ...string) (*exec.Cmd, func() error, error) {
	f, err := fs.Open(filename)
	if err != nil {
		return &exec.Cmd{}, nil, errors.WithStack(err)
	}
	defer f.Close()

	mfd, err := memfd.Create()
	if err != nil {
		return &exec.Cmd{}, nil, errors.WithStack(err)
	}
	defer mfd.Unmap()

	_, err = io.Copy(mfd, f)
	if err != nil {
		mfd.Close()
		return &exec.Cmd{}, nil, errors.WithStack(err)
	}

	cmd := exec.Command(fmt.Sprintf("/proc/self/fd/%d", mfd.Fd()), args...)
	return cmd, mfd.Close, nil
}

func ExtractCommand(fs embed.FS, filename, dest string) error {
	f, err := fs.Open(filename)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	dfd, err := os.Create(dest)
	if err != nil {
		return errors.WithStack(err)
	}
	defer dfd.Close()

	_, err = io.Copy(dfd, f)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
