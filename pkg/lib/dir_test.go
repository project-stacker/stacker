package lib_test

import (
	"os"
	"path"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"stackerbuild.io/stacker/pkg/lib"
)

func TestDir(t *testing.T) {
	Convey("IsSymLink", t, func() {
		src, err := os.CreateTemp("", "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		_, err = src.Write([]byte("hello world!"))
		So(err, ShouldBeNil)

		ok, _ := lib.IsSymlink(src.Name())
		So(ok, ShouldBeFalse)
	})

	Convey("DirCopy", t, func() {
		src, err := os.CreateTemp("", "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		_, err = src.Write([]byte("hello world!"))
		So(err, ShouldBeNil)

		dest, err := os.MkdirTemp("", "dest")
		So(err, ShouldBeNil)
		So(dest, ShouldNotBeNil)
		defer os.Remove(dest)

		err = lib.DirCopy(path.Dir(src.Name()), dest)
		So(err, ShouldBeNil)
	})

	Convey("CopyThing", t, func() {
		src, err := os.CreateTemp("", "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		_, err = src.Write([]byte("hello world!"))
		So(err, ShouldBeNil)

		dest, err := os.CreateTemp("", "dest")
		So(err, ShouldBeNil)
		So(dest, ShouldNotBeNil)
		defer os.Remove(dest.Name())

		err = lib.CopyThing(src.Name(), dest.Name())
		So(err, ShouldBeNil)
	})

	Convey("Chmod", t, func() {
		src, err := os.CreateTemp("", "src")
		So(err, ShouldBeNil)
		So(src, ShouldNotBeNil)
		defer os.Remove(src.Name())

		_, err = src.Write([]byte("hello world!"))
		So(err, ShouldBeNil)

		err = lib.Chmod("644", src.Name())
		So(err, ShouldBeNil)
	})
}
