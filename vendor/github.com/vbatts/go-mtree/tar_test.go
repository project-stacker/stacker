package mtree

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func ExampleStreamer() {
	fh, err := os.Open("./testdata/test.tar")
	if err != nil {
		// handle error ...
	}
	str := NewTarStreamer(fh, nil, nil)
	if err := extractTar("/tmp/dir", str); err != nil {
		// handle error ...
	}

	dh, err := str.Hierarchy()
	if err != nil {
		// handle error ...
	}

	res, err := Check("/tmp/dir/", dh, nil, nil)
	if err != nil {
		// handle error ...
	}
	if len(res) > 0 {
		// handle validation issue ...
	}
}
func extractTar(root string, tr io.Reader) error {
	return nil
}

func TestTar(t *testing.T) {
	/*
		data, err := makeTarStream()
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBuffer(data)
		str := NewTarStreamer(buf, append(DefaultKeywords, "sha1"))
	*/
	/*
		// open empty folder and check size.
		fh, err := os.Open("./testdata/empty")
		if err != nil {
			t.Fatal(err)
		}
		log.Println(fh.Stat())
		fh.Close() */
	fh, err := os.Open("./testdata/test.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, nil, append(DefaultKeywords, "sha1"))

	if _, err := io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err := str.Close(); err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	// get DirectoryHierarcy struct from walking the tar archive
	tdh, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}
	if tdh == nil {
		t.Fatal("expected a DirectoryHierarchy struct, but got nil")
	}

	testDir, present := os.LookupEnv("MTREE_TESTDIR")
	if present == false {
		testDir = "."
	}
	testPath := filepath.Join(testDir, "test.mtree")
	fh, err = os.Create(testPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testPath)

	// put output of tar walk into test.mtree
	_, err = tdh.WriteTo(fh)
	if err != nil {
		t.Fatal(err)
	}
	fh.Close()

	// now simulate gomtree -T testdata/test.tar -f testdata/test.mtree
	fh, err = os.Open(testPath)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	dh, err := ParseSpec(fh)
	if err != nil {
		t.Fatal(err)
	}

	res, err := TarCheck(tdh, dh, append(DefaultKeywords, "sha1"))
	if err != nil {
		t.Fatal(err)
	}

	// print any failures, and then call t.Fatal once all failures/extra/missing
	// are outputted
	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
		t.Fatal("unexpected errors")
	}
}

// This test checks how gomtree handles archives that were created
// with multiple directories, i.e, archives created with something like:
// `tar -cvf some.tar dir1 dir2 dir3 dir4/dir5 dir6` ... etc.
// The testdata of collection.tar resemble such an archive. the `collection` folder
// is the contents of `collection.tar` extracted
func TestArchiveCreation(t *testing.T) {
	fh, err := os.Open("./testdata/collection.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, nil, []Keyword{"sha1"})

	if _, err := io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err := str.Close(); err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	// get DirectoryHierarcy struct from walking the tar archive
	tdh, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}

	// Test the tar manifest against the actual directory
	res, err := Check("./testdata/collection", tdh, []Keyword{"sha1"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
		t.Fatal("unexpected errors")
	}

	// Test the tar manifest against itself
	res, err = TarCheck(tdh, tdh, []Keyword{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
		t.Fatal("unexpected errors")
	}

	// Validate the directory manifest against the archive
	dh, err := Walk("./testdata/collection", nil, []Keyword{"sha1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = TarCheck(tdh, dh, []Keyword{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
		t.Fatal("unexpected errors")
	}
}

// Now test a tar file that was created with just the path to a file. In this
// test case, the traversal and creation of "placeholder" directories are
// evaluated. Also, The fact that this archive contains a single entry, yet the
// entry is associated with a file that has parent directories, means that the
// "." directory should be the lowest sub-directory under which `file` is contained.
func TestTreeTraversal(t *testing.T) {
	fh, err := os.Open("./testdata/traversal.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, nil, DefaultTarKeywords)

	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}

	fh.Close()
	tdh, err := str.Hierarchy()

	if err != nil {
		t.Fatal(err)
	}

	res, err := TarCheck(tdh, tdh, []Keyword{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		for _, delta := range res {
			t.Error(delta)
		}
		t.Fatal("unexpected errors")
	}

	// top-level "." directory will contain contents of traversal.tar
	res, err = Check("./testdata/.", tdh, []Keyword{"sha1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		var failed bool
		for _, delta := range res {
			// We only care about missing or modified files.
			// The original test was written using the old check code.
			if delta.Type() != Extra {
				failed = true
				t.Error(delta)
			}
		}
		if failed {
			t.Fatal("unexpected errors")
		}
	}

	// Now test an archive that requires placeholder directories, i.e, there are
	// no headers in the archive that are associated with the actual directory name
	fh, err = os.Open("./testdata/singlefile.tar")
	if err != nil {
		t.Fatal(err)
	}
	str = NewTarStreamer(fh, nil, DefaultTarKeywords)
	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}
	tdh, err = str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}

	// Implied top-level "." directory will contain the contents of singlefile.tar
	res, err = Check("./testdata/.", tdh, []Keyword{"sha1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 0 {
		var failed bool
		for _, delta := range res {
			// We only care about missing or modified files.
			// The original test was written using the old check code.
			if delta.Type() != Extra {
				failed = true
				t.Error(delta)
			}
		}
		if failed {
			t.Fatal("unexpected errors")
		}
	}
}

func TestHardlinks(t *testing.T) {
	fh, err := os.Open("./testdata/hardlinks.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, nil, append(DefaultTarKeywords, "nlink"))

	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}

	fh.Close()
	tdh, err := str.Hierarchy()

	if err != nil {
		t.Fatal(err)
	}
	foundnlink := false
	for _, e := range tdh.Entries {
		if e.Type == RelativeType {
			for _, kv := range e.Keywords {
				if KeyVal(kv).Keyword() == "nlink" {
					foundnlink = true
					if KeyVal(kv).Value() != "3" {
						t.Errorf("expected to have 3 hardlinks for %s", e.Name)
					}
				}
			}
		}
	}
	if !foundnlink {
		t.Errorf("nlink expected to be evaluated")
	}
}

type fakeFile struct {
	Name, Body string
	Mode       int64
	Type       byte
	Sec, Nsec  int64
	Xattrs     map[string]string
}

// minimal tar archive that mimics what is in ./testdata/test.tar
var minimalFiles = []fakeFile{
	{"x/", "", 0755, '5', 0, 0, nil},
	{"x/files", "howdy\n", 0644, '0', 0, 0, nil},
}

func makeTarStream(ff []fakeFile) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Add some files to the archive.
	for _, file := range ff {
		hdr := &tar.Header{
			Name:       file.Name,
			Uid:        syscall.Getuid(),
			Gid:        syscall.Getgid(),
			Mode:       file.Mode,
			Typeflag:   file.Type,
			Size:       int64(len(file.Body)),
			ModTime:    time.Unix(file.Sec, file.Nsec),
			AccessTime: time.Unix(file.Sec, file.Nsec),
			ChangeTime: time.Unix(file.Sec, file.Nsec),
			Xattrs:     file.Xattrs,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if len(file.Body) > 0 {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				return nil, err
			}
		}
	}
	// Make sure to check the error on Close.
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestArchiveExcludeNonDirectory(t *testing.T) {
	fh, err := os.Open("./testdata/collection.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, []ExcludeFunc{ExcludeNonDirectories}, []Keyword{"type"})

	if _, err := io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err := str.Close(); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	// get DirectoryHierarcy struct from walking the tar archive
	tdh, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}
	for i := range tdh.Entries {
		for _, keyval := range tdh.Entries[i].AllKeys() {
			if tdh.Entries[i].Type == FullType || tdh.Entries[i].Type == RelativeType {
				if keyval.Keyword() == "type" && keyval.Value() != "dir" {
					t.Errorf("expected only directories, but %q is a %q", tdh.Entries[i].Name, keyval.Value())
				}
			}
		}
	}
}
