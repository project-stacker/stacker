package stacker

import (
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
)

// download with caching support in the specified cache dir.
func Download(cacheDir string, url string, progress bool, expectedHash, remoteHash, remoteSize string,
	mode *fs.FileMode, uid, gid int,
) (string, error) {
	name := path.Join(cacheDir, path.Base(url))

	if fi, err := os.Stat(name); err == nil {
		// Couldn't get remoteHash then use cached copy of import
		if remoteHash == "" {
			log.Infof("Couldn't obtain file info of %s, using cached copy", url)
			return name, nil
		}
		// File is found in cache
		// need to check if cache is valid before using it
		localHash, err := lib.HashFile(name, false)
		if err != nil {
			return "", err
		}
		localHash = strings.TrimPrefix(localHash, "sha256:")
		localSize := strconv.FormatInt(fi.Size(), 10)
		log.Debugf("Local file: hash: %s length: %s", localHash, localSize)

		if localHash == remoteHash {
			// Cached file has same hash as the remote file
			log.Infof("matched hash of %s, using cached copy", url)
			return name, nil
		} else if localSize == remoteSize {
			// Cached file has same content length as the remote file
			log.Infof("matched content length of %s, taking a leap of faith and using cached copy", url)
			return name, nil
		}
		// Cached file has a different hash from the remote one
		// Need to cleanup
		err = os.RemoveAll(name)
		if err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		// File is not found in cache but there are other errors
		return "", err
	}

	// File is not in cache
	// it wasn't there in the first place or it was cleaned up
	out, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer out.Close()

	log.Infof("downloading %v", url)

	resp, err := http.Get(url)
	if err != nil {
		os.RemoveAll(name)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		os.RemoveAll(name)
		return "", errors.Errorf("couldn't download %s: %s", url, resp.Status)
	}

	source := resp.Body
	if progress {
		bar := pb.New(int(resp.ContentLength)).Set(pb.Bytes, true)
		bar.Start()
		source = bar.NewProxyReader(source)
		defer bar.Finish()
	}

	_, err = io.Copy(out, source)

	if err != nil {
		return "", err
	}
	if expectedHash != "" {
		log.Infof("Checking shasum of downloaded file")

		downloadHash, err := lib.HashFile(name, false)
		if err != nil {
			return "", err
		}

		downloadHash = strings.TrimPrefix(downloadHash, "sha256:")
		log.Debugf("Downloaded file hash: %s", downloadHash)

		if expectedHash != downloadHash {
			os.RemoveAll(name)
			return "", errors.Errorf("Downloaded file hash does not match. Expected: %s Actual: %s", expectedHash, downloadHash)
		}
	}

	if mode != nil {
		err = out.Chmod(*mode)
		if err != nil {
			return "", errors.Wrapf(err, "Coudn't chmod file %s", name)
		}
	}

	err = out.Chown(uid, gid)
	if err != nil {
		return "", errors.Wrapf(err, "Coudn't chown file %s", source)
	}

	return name, err
}

// getHttpFileInfo returns the hash and content size a file stored on a web server
func getHttpFileInfo(remoteURL string) (string, string, error) {

	// Verify URL scheme
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", errors.Errorf("cannot obtain content info for non HTTP URL: (%s)", remoteURL)
	}

	// Make a HEAD call on remote URL
	resp, err := http.Head(remoteURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// Get file info from header
	// If the hash is not present this is an empty string
	hash := resp.Header.Get("X-Checksum-Sha256")
	length := resp.Header.Get("Content-Length")

	return hash, length, nil
}
