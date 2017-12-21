// +build !linux

package xattr

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestXattrUnsupported(t *testing.T) {
	fh, err := ioutil.TempFile(".", "xattr.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fh.Name())
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}

	// because xattrs are "not supported" on this platform, they're like a black
	// box.
	write := []byte("1234")
	expected := []byte("")

	if err := Set(fh.Name(), "user.testing", write); err != nil {
		t.Fatal(fh.Name(), err)
	}
	l, err := List(fh.Name())
	if err != nil {
		t.Error(fh.Name(), err)
	}
	if len(l) > 0 {
		t.Errorf("%q: expected a list of at least 0; got %d", fh.Name(), len(l))
	}
	got, err := Get(fh.Name(), "user.testing")
	if err != nil {
		t.Fatal(fh.Name(), err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("%q: expected %q; got %q", fh.Name(), expected, got)
	}
}
