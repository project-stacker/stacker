package lib

import (
	"fmt"
	"io"
	"os"

	"github.com/minio/sha256-simd"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func HashFile(path string, includeMode bool) (string, error) {
	h := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't open %s for hashing", path)
	}
	defer f.Close()

	_, err = io.Copy(h, f)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't copy %s for hashing", path)
	}

	if includeMode {
		// Include file mode when computing the hash
		// In general we want to do this, but not all external
		// tooling includes it, so we can't compare it with the hash
		// in the reply of a HTTP HEAD call

		fi, err := f.Stat()
		if err != nil {
			return "", errors.Wrapf(err, "couldn't stat %s for hashing", path)
		}

		_, err = h.Write([]byte(fmt.Sprintf("%v", fi.Mode())))
		if err != nil {
			return "", errors.Wrapf(err, "couldn't write mode")
		}
	}

	d := digest.NewDigest("sha256", h)
	return d.String(), nil
}
