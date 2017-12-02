package stacker

import (
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"path"
)

func fileCopy(dest string, source string) error {
	s, err := os.Open(source)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func Import(c StackerConfig, name string, imports []string) error {
	dir := path.Join(c.StackerDir, "imports", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, i := range imports {
		url, err := url.Parse(i)
		if err != nil {
			return err
		}

		encoded := base64.URLEncoding.EncodeToString([]byte(i))

		// It's just a path, let's copy it to .stacker.
		if url.Scheme == "" {
			if err := fileCopy(path.Join(dir, encoded), i); err != nil {
				return err
			}
		} else {
			// otherwise, we need to download it
			_, err = download(dir, i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
