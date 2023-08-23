package lib_test

import (
	"io/fs"
	"os"
	"path"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"stackerbuild.io/stacker/pkg/lib"
)

func TestFile(t *testing.T) {
	Convey("FileCopy", t, func() {
		src, err := os.CreateTemp("", "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		_, err = src.Write([]byte("hello world!"))
		So(err, ShouldBeNil)

		Convey("With defaults", func() {
			dest, err := os.CreateTemp("", "dest")
			So(err, ShouldBeNil)
			defer os.Remove(dest.Name())

			err = lib.FileCopy(dest.Name(), src.Name(), nil, -1, -1)
			So(err, ShouldBeNil)
		})

		Convey("With non-default mode", func() {
			dest, err := os.CreateTemp("", "dest")
			So(err, ShouldBeNil)
			defer os.Remove(dest.Name())

			mode := fs.FileMode(0644)
			err = lib.FileCopy(dest.Name(), src.Name(), &mode, -1, -1)
			So(err, ShouldBeNil)
		})
	})

	Convey("FindFiles", t, func() {
		tdir, err := os.MkdirTemp("", "find-files-test-*")
		So(err, ShouldBeNil)
		So(tdir, ShouldNotBeNil)
		defer os.RemoveAll(tdir)

		src, err := os.CreateTemp(tdir, "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		files, err := lib.FindFiles(path.Dir(src.Name()), ".*")
		So(err, ShouldBeNil)
		So(files, ShouldNotBeEmpty)
	})
}
