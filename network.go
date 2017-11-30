package stacker

import (
	"path"
	"encoding/base64"
	"os"
	"net/http"
	"io"
)

// download with caching support in the specified cache dir.
func download(cacheDir string, url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	name := path.Join(cacheDir, base64.URLEncoding.EncodeToString([]byte(url)))
	out, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// It already exists, let's just use that one.
		if os.IsExist(err) {
			return name, nil
		}
		return "", err
	}

	_, err = io.Copy(out, resp.Body)
	return name, err
}
