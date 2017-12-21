package mtree

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var mockTime = time.Unix(1337888823, 0)

// Here be some dodgy testing. In particular, we have to mess around with some
// of the FsEval functions. In particular, we change all of the FileInfos to a
// different value.

type mockFileInfo struct {
	os.FileInfo
}

func (fi mockFileInfo) Mode() os.FileMode {
	return os.FileMode(fi.FileInfo.Mode() | 0777)
}

func (fi mockFileInfo) ModTime() time.Time {
	return mockTime
}

type MockFsEval struct {
	open, lstat, readdir, keywordFunc int
}

// Open must have the same semantics as os.Open.
func (fs *MockFsEval) Open(path string) (*os.File, error) {
	fs.open++
	return os.Open(path)
}

// Lstat must have the same semantics as os.Lstat.
func (fs *MockFsEval) Lstat(path string) (os.FileInfo, error) {
	fs.lstat++
	fi, err := os.Lstat(path)
	return mockFileInfo{fi}, err
}

// Readdir must have the same semantics as calling os.Open on the given
// path and then returning the result of (*os.File).Readdir(-1).
func (fs *MockFsEval) Readdir(path string) ([]os.FileInfo, error) {
	fs.readdir++
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	fis, err := fh.Readdir(-1)
	if err != nil {
		return nil, err
	}
	for idx := range fis {
		fis[idx] = mockFileInfo{fis[idx]}
	}
	return fis, nil
}

// KeywordFunc must return a wrapper around the provided function (in other
// words, the returned function must refer to the same keyword).
func (fs *MockFsEval) KeywordFunc(fn KeywordFunc) KeywordFunc {
	fs.keywordFunc++
	return fn
}

func TestCheckFsEval(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-check-fs-eval")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	content := []byte("If you hide your ignorance, no one will hit you and you'll never learn.")
	tmpfn := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfn, content, 0451); err != nil {
		t.Fatal(err)
	}

	// Walk this tempdir
	mock := &MockFsEval{}
	dh, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), mock)
	if err != nil {
		t.Fatal(err)
	}
	// Make sure that mock functions have been called.
	if mock.open == 0 {
		t.Errorf("mock.Open not called")
	}
	if mock.lstat == 0 {
		t.Errorf("mock.Lstat not called")
	}
	if mock.readdir == 0 {
		t.Errorf("mock.Readdir not called")
	}
	if mock.keywordFunc == 0 {
		t.Errorf("mock.KeywordFunc not called")
	}

	// Check for sanity. This ought to pass.
	mock = &MockFsEval{}
	res, err := Check(dir, dh, nil, mock)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		t.Errorf("%#v", res)
	}
	// Make sure that mock functions have been called.
	if mock.open == 0 {
		t.Errorf("mock.Open not called")
	}
	if mock.lstat == 0 {
		t.Errorf("mock.Lstat not called")
	}
	if mock.readdir == 0 {
		t.Errorf("mock.Readdir not called")
	}
	if mock.keywordFunc == 0 {
		t.Errorf("mock.KeywordFunc not called")
	}

	// This should FAIL.
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Errorf("expected Check to fail")
	}

	// Modify the metadata so you can get the right output.
	if err := os.Chmod(tmpfn, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmpfn, mockTime, mockTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, mockTime, mockTime); err != nil {
		t.Fatal(err)
	}

	// It should now succeed.
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		buf, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			t.Errorf("%#v", res)
		} else {
			t.Errorf("%s", buf)
		}
	}
}
