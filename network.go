package stacker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/cheggaaa/pb"
)

// download with caching support in the specified cache dir.
func download(cacheDir string, url string) (string, error) {
	name := path.Join(cacheDir, path.Base(url))
	out, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// It already exists, let's just use that one.
		if os.IsExist(err) {
			fmt.Println("using cached copy of", url)
			return name, nil
		} else if os.IsNotExist(err) {
			out, err = os.OpenFile(name, os.O_RDWR, 0644)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	defer out.Close()

	fmt.Println("downloading", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("couldn't download %s: %s", url, resp.Status)
	}

	source := resp.Body
	if resp.ContentLength >= 0 {
		bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
		bar.ShowTimeLeft = true
		bar.ShowSpeed = true
		bar.Start()
		source = bar.NewProxyReader(source)
		defer bar.Finish()
	}

	_, err = io.Copy(out, source)
	return name, err
}
