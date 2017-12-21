package mtree

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// simple walk of current directory, and imediately check it.
// may not be parallelizable.
func TestCheck(t *testing.T) {
	dh, err := Walk(".", nil, append(DefaultKeywords, []Keyword{"sha1", "xattr"}...), nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := Check(".", dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(res) > 0 {
		t.Errorf("%#v", res)
	}
}

// make a directory, walk it, check it, modify the timestamp and ensure it fails.
// only check again for size and sha1, and ignore time, and ensure it passes
func TestCheckKeywords(t *testing.T) {
	content := []byte("I know half of you half as well as I ought to")
	dir, err := ioutil.TempDir("", "test-check-keywords")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		t.Fatal(err)
	}

	// Walk this tempdir
	dh, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check for sanity. This ought to pass.
	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		t.Errorf("%#v", res)
	}

	// Touch a file, so the mtime changes.
	newtime := time.Date(2006, time.February, 1, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(tmpfn, newtime, newtime); err != nil {
		t.Fatal(err)
	}

	// Check again. This ought to fail.
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatal("expected to get 1 delta on changed mtimes, but did not")
	}
	if res[0].Type() != Modified {
		t.Errorf("expected to get modified delta on changed mtimes, but did not")
	}

	// Check again, but only sha1 and mode. This ought to pass.
	res, err = Check(dir, dh, []Keyword{"sha1", "mode"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		t.Errorf("%#v", res)
	}
}

func ExampleCheck() {
	dh, err := Walk(".", nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		// handle error ...
	}

	res, err := Check(".", dh, nil, nil)
	if err != nil {
		// handle error ...
	}
	if len(res) > 0 {
		// handle failed validity ...
	}
}

// Tests default action for evaluating a symlink, which is just to compare the
// link itself, not to follow it
func TestDefaultBrokenLink(t *testing.T) {
	dh, err := Walk("./testdata/dirwithbrokenlink", nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Check("./testdata/dirwithbrokenlink", dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
	}
}

// https://github.com/vbatts/go-mtree/issues/8
func TestTimeComparison(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-time.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// This is the format of time from FreeBSD
	spec := `
/set type=file time=5.000000000
.               type=dir
    file       time=5.000000000
..
`

	fh, err := os.Create(filepath.Join(dir, "file"))
	if err != nil {
		t.Fatal(err)
	}
	// This is what mode we're checking for. Round integer of epoch seconds
	epoch := time.Unix(5, 0)
	if err := os.Chtimes(fh.Name(), epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Error(err)
	}

	dh, err := ParseSpec(bytes.NewBufferString(spec))
	if err != nil {
		t.Fatal(err)
	}

	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Error(err)
	}
	if len(res) > 0 {
		t.Fatal(res)
	}
}

func TestTarTime(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-tar-time.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// This is the format of time from FreeBSD
	spec := `
/set type=file time=5.454353132
.               type=dir time=5.123456789
    file       time=5.911134111
..
`

	fh, err := os.Create(filepath.Join(dir, "file"))
	if err != nil {
		t.Fatal(err)
	}
	// This is what mode we're checking for. Round integer of epoch seconds
	epoch := time.Unix(5, 0)
	if err := os.Chtimes(fh.Name(), epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Error(err)
	}

	dh, err := ParseSpec(bytes.NewBufferString(spec))
	if err != nil {
		t.Fatal(err)
	}

	keywords := dh.UsedKeywords()

	// make sure "time" keyword works
	_, err = Check(dir, dh, keywords, nil)
	if err != nil {
		t.Error(err)
	}

	// make sure tar_time wins
	res, err := Check(dir, dh, append(keywords, "tar_time"), nil)
	if err != nil {
		t.Error(err)
	}
	if len(res) > 0 {
		t.Fatal(res)
	}
}

func TestIgnoreComments(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-comments.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// This is the format of time from FreeBSD
	spec := `
/set type=file time=5.000000000
.               type=dir
    file1       time=5.000000000
..
`

	fh, err := os.Create(filepath.Join(dir, "file1"))
	if err != nil {
		t.Fatal(err)
	}
	// This is what mode we're checking for. Round integer of epoch seconds
	epoch := time.Unix(5, 0)
	if err := os.Chtimes(fh.Name(), epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, epoch, epoch); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Error(err)
	}

	dh, err := ParseSpec(bytes.NewBufferString(spec))
	if err != nil {
		t.Fatal(err)
	}

	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Error(err)
	}

	if len(res) > 0 {
		t.Fatal(res)
	}

	// now change the spec to a comment that looks like an actual Entry but has
	// whitespace in front of it
	spec = `
/set type=file time=5.000000000
.               type=dir
    file1       time=5.000000000
	#file2 		time=5.000000000
..
`
	dh, err = ParseSpec(bytes.NewBufferString(spec))

	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Error(err)
	}

	if len(res) > 0 {
		t.Fatal(res)
	}
}

func TestCheckNeedsEncoding(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-needs-encoding")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	fh, err := os.Create(filepath.Join(dir, "file[ "))
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Error(err)
	}
	fh, err = os.Create(filepath.Join(dir, "    , should work"))
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Error(err)
	}

	dh, err := Walk(dir, nil, DefaultKeywords, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		t.Fatal(res)
	}
}
