package stacker

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/udhos/equalfile"
)

type diffFunc func(path1 string, info1 os.FileInfo, path2 string, info2 os.FileInfo) error

func compareFiles(p1 string, info1 os.FileInfo, p2 string, info2 os.FileInfo) (bool, error) {
	if info1.Name() != info2.Name() {
		return false, fmt.Errorf("comparing files without the same name?")
	}

	if info1.IsDir() {
		if info2.IsDir() {
			return false, nil
		}

		return false, fmt.Errorf("adding new directory where file was not current supported")
	}

	if info1.Mode()&os.ModeSymlink != 0 {
		if info2.Mode()&os.ModeSymlink != 0 {
			link1, err := os.Readlink(p1)
			if err != nil {
				return false, err
			}

			link2, err := os.Readlink(p2)
			if err != nil {
				return false, err
			}
			return link1 != link2, err
		}

		return false, fmt.Errorf("symlink -> not symlink not supported")
	}

	if info1.Size() != info2.Size() {
		return true, nil
	}

	f1, err := os.Open(p1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(p2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	return equalfile.New(nil, equalfile.Options{}).CompareReader(f1, f2)
}

func directoryDiff(path1 string, path2 string, diff diffFunc) error {
	dir1, err := ioutil.ReadDir(path1)
	if err != nil {
		return err
	}

	dir2, err := ioutil.ReadDir(path2)
	if err != nil {
		return err
	}

	for _, e1 := range dir1 {
		found := false
		p1 := path.Join(path1, e1.Name())

		for _, e2 := range dir2 {
			p2 := path.Join(path2, e2.Name())
			if e1.Name() == e2.Name() {
				different, err := compareFiles(p1, e1, p2, e2)
				if err != nil {
					return err
				}

				if different {
					if err := diff(p1, e1, p2, e2); err != nil {
						return err
					}
				}

				found = true
				break
			}
		}

		if !found {
			p := path.Join(path1, e1.Name())
			err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				return diff(path, info, "", nil)
			})
			if err != nil {
				return err
			}
		}
	}

	for _, e2 := range dir2 {
		found := false

		// only check for deleted files this time; we already did diff above
		for _, e1 := range dir1 {
			if e1.Name() == e2.Name() {
				found = true
				break
			}
		}

		if !found {
			p := path.Join(path2, e2.Name())
			err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				return diff("", nil, path, info)
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type chanReader struct {
	ch  chan []byte
	cur io.Reader
}

func (r *chanReader) Read(p []byte) (int, error) {
	if r.cur == nil {
		bs, ok := <-r.ch
		if !ok {
			return 0, io.EOF
		}

		r.cur = bytes.NewReader(bs)
	}

	n, err := r.cur.Read(p)
	if err == io.EOF {
		r.cur = nil
		err = nil
	}
	return n, err
}

func buildTarEntry(target string, path string, info os.FileInfo) (*tar.Header, io.ReadCloser, error) {
	var content io.ReadCloser
	var err error

	// nothing needed for directories
	if info.IsDir() {
		return nil, nil, nil
	}

	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return nil, nil, err
		}
	} else {
		content, err = os.Open(path)
		if err != nil {
			return nil, nil, err
		}
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		content.Close()
		return nil, nil, err
	}

	// fix up the path
	header.Name = path[len(target):]

	return header, content, nil
}

func doTarDiff(source, target string, tw *tar.Writer) error {
	diffFunc := func(path1 string, info1 os.FileInfo, path2 string, info2 os.FileInfo) error {
		var header *tar.Header
		var content io.ReadCloser

		fmt.Println("got diff of", path1, path2)

		// remove the file
		if path2 == "" {
			whiteout := path.Join(path.Dir(path1[len(source):]), fmt.Sprintf(".wh.%s", info1.Name()))
			header = &tar.Header{
				Name:     whiteout,
				Mode:     0644,
				Typeflag: tar.TypeReg,
			}
			//fmt.Printf("added whiteout file %s\n", header.Name)
		} else {
			// the file added or was changed, copy the v2 version in
			var err error
			header, content, err = buildTarEntry(target, path2, info2)
			if err != nil {
				return err
			}
			if content != nil {
				defer content.Close()
			}
		}

		if header != nil {
			err := tw.WriteHeader(header)
			if err != nil {
				return err
			}

			if content != nil {
				_, err := io.Copy(tw, content)
				return err
			}
		}

		return nil

	}

	return directoryDiff(source, target, diffFunc)
}

func tarDiff(config StackerConfig, source string, target string) (io.ReadCloser, hash.Hash, error) {
	r, w := io.Pipe()
	gz := gzip.NewWriter(w)
	h := sha256.New()
	tw := tar.NewWriter(io.MultiWriter(gz, h))
	s := path.Join(config.RootFSDir, source)
	t := path.Join(config.RootFSDir, target)
	if source == "" {
		go func() {
			err := filepath.Walk(t, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				fmt.Println("adding", path)

				header, content, err := buildTarEntry(t, path, info)
				if err != nil {
					return err
				}

				if content != nil {
					defer content.Close()
				}

				if header != nil {
					err := tw.WriteHeader(header)
					if err != nil {
						return err
					}

					if content != nil {
						_, err := io.Copy(tw, content)
						return err
					}
				}

				return nil

			})
			tw.Close()
			gz.Close()
			w.CloseWithError(err)
		}()
	} else {
		go func() {
			err := doTarDiff(s, t, tw)
			tw.Close()
			gz.Close()
			w.CloseWithError(err)
		}()
	}
	return r, h, nil
}
