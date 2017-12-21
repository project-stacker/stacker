package mtree

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// simple walk of current directory, and imediately check it.
// may not be parallelizable.
func TestCompare(t *testing.T) {
	old, err := Walk(".", nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	new, err := Walk(".", nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	diffs, err := Compare(old, new, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(diffs) > 0 {
		t.Errorf("%#v", diffs)
	}
}

func TestCompareModified(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-compare-modified")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a bunch of objects.
	tmpfile := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfile, []byte("some content here"), 0666); err != nil {
		t.Fatal(err)
	}

	tmpdir := filepath.Join(dir, "testdir")
	if err := os.Mkdir(tmpdir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpsubfile := filepath.Join(tmpdir, "anotherfile")
	if err := ioutil.WriteFile(tmpsubfile, []byte("some different content"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the current state.
	old, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite the content in one of the files.
	if err := ioutil.WriteFile(tmpsubfile, []byte("modified content"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the new state.
	new, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Compare.
	diffs, err := Compare(old, new, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1 object
	if len(diffs) != 1 {
		t.Errorf("expected the diff length to be 1, got %d", len(diffs))
		for i, diff := range diffs {
			t.Logf("diff[%d] = %#v", i, diff)
		}
	}

	// These cannot fail.
	tmpfile, _ = filepath.Rel(dir, tmpfile)
	tmpdir, _ = filepath.Rel(dir, tmpdir)
	tmpsubfile, _ = filepath.Rel(dir, tmpsubfile)

	for _, diff := range diffs {
		switch diff.Path() {
		case tmpsubfile:
			if diff.Type() != Modified {
				t.Errorf("unexpected diff type for %s: %s", diff.Path(), diff.Type())
			}

			if diff.Diff() == nil {
				t.Errorf("expect to not get nil for .Diff()")
			}

			old := diff.Old()
			new := diff.New()
			if old == nil || new == nil {
				t.Errorf("expected to get (!nil, !nil) for (.Old, .New), got (%#v, %#v)", old, new)
			}
		default:
			t.Errorf("unexpected diff found: %#v", diff)
		}
	}
}

func TestCompareMissing(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-compare-missing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a bunch of objects.
	tmpfile := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfile, []byte("some content here"), 0666); err != nil {
		t.Fatal(err)
	}

	tmpdir := filepath.Join(dir, "testdir")
	if err := os.Mkdir(tmpdir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpsubfile := filepath.Join(tmpdir, "anotherfile")
	if err := ioutil.WriteFile(tmpsubfile, []byte("some different content"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the current state.
	old, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the objects.
	if err := os.RemoveAll(tmpfile); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(tmpsubfile); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(tmpdir); err != nil {
		t.Fatal(err)
	}

	// Walk the new state.
	new, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Compare.
	diffs, err := Compare(old, new, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 3 objects + the changes to '.'
	if len(diffs) != 4 {
		t.Errorf("expected the diff length to be 4, got %d", len(diffs))
		for i, diff := range diffs {
			t.Logf("diff[%d] = %#v", i, diff)
		}
	}

	// These cannot fail.
	tmpfile, _ = filepath.Rel(dir, tmpfile)
	tmpdir, _ = filepath.Rel(dir, tmpdir)
	tmpsubfile, _ = filepath.Rel(dir, tmpsubfile)

	for _, diff := range diffs {
		switch diff.Path() {
		case ".":
			// ignore these changes
		case tmpfile, tmpdir, tmpsubfile:
			if diff.Type() != Missing {
				t.Errorf("unexpected diff type for %s: %s", diff.Path(), diff.Type())
			}

			if diff.Diff() != nil {
				t.Errorf("expect to get nil for .Diff(), got %#v", diff.Diff())
			}

			old := diff.Old()
			new := diff.New()
			if old == nil || new != nil {
				t.Errorf("expected to get (!nil, nil) for (.Old, .New), got (%#v, %#v)", old, new)
			}
		default:
			t.Errorf("unexpected diff found: %#v", diff)
		}
	}
}

func TestCompareExtra(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-compare-extra")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Walk the current state.
	old, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a bunch of objects.
	tmpfile := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfile, []byte("some content here"), 0666); err != nil {
		t.Fatal(err)
	}

	tmpdir := filepath.Join(dir, "testdir")
	if err := os.Mkdir(tmpdir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpsubfile := filepath.Join(tmpdir, "anotherfile")
	if err := ioutil.WriteFile(tmpsubfile, []byte("some different content"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the new state.
	new, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Compare.
	diffs, err := Compare(old, new, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 3 objects + the changes to '.'
	if len(diffs) != 4 {
		t.Errorf("expected the diff length to be 4, got %d", len(diffs))
		for i, diff := range diffs {
			t.Logf("diff[%d] = %#v", i, diff)
		}
	}

	// These cannot fail.
	tmpfile, _ = filepath.Rel(dir, tmpfile)
	tmpdir, _ = filepath.Rel(dir, tmpdir)
	tmpsubfile, _ = filepath.Rel(dir, tmpsubfile)

	for _, diff := range diffs {
		switch diff.Path() {
		case ".":
			// ignore these changes
		case tmpfile, tmpdir, tmpsubfile:
			if diff.Type() != Extra {
				t.Errorf("unexpected diff type for %s: %s", diff.Path(), diff.Type())
			}

			if diff.Diff() != nil {
				t.Errorf("expect to get nil for .Diff(), got %#v", diff.Diff())
			}

			old := diff.Old()
			new := diff.New()
			if old != nil || new == nil {
				t.Errorf("expected to get (!nil, nil) for (.Old, .New), got (%#v, %#v)", old, new)
			}
		default:
			t.Errorf("unexpected diff found: %#v", diff)
		}
	}
}

func TestCompareKeys(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-compare-keys")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a bunch of objects.
	tmpfile := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfile, []byte("some content here"), 0666); err != nil {
		t.Fatal(err)
	}

	tmpdir := filepath.Join(dir, "testdir")
	if err := os.Mkdir(tmpdir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpsubfile := filepath.Join(tmpdir, "anotherfile")
	if err := ioutil.WriteFile(tmpsubfile, []byte("aaa"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the current state.
	old, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite the content in one of the files, but without changing the size.
	if err := ioutil.WriteFile(tmpsubfile, []byte("bbb"), 0666); err != nil {
		t.Fatal(err)
	}

	// Walk the new state.
	new, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Compare.
	diffs, err := Compare(old, new, []Keyword{"size"})
	if err != nil {
		t.Fatal(err)
	}

	// 0 objects
	if len(diffs) != 0 {
		t.Errorf("expected the diff length to be 0, got %d", len(diffs))
		for i, diff := range diffs {
			t.Logf("diff[%d] = %#v", i, diff)
		}
	}
}

func TestTarCompare(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-compare-tar")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a bunch of objects.
	tmpfile := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfile, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	tmpdir := filepath.Join(dir, "testdir")
	if err := os.Mkdir(tmpdir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpsubfile := filepath.Join(tmpdir, "anotherfile")
	if err := ioutil.WriteFile(tmpsubfile, []byte("aaa"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a tar-like archive.
	compareFiles := []fakeFile{
		{"./", "", 0700, tar.TypeDir, 100, 0, nil},
		{"tmpfile", "some content", 0644, tar.TypeReg, 100, 0, nil},
		{"testdir/", "", 0755, tar.TypeDir, 100, 0, nil},
		{"testdir/anotherfile", "aaa", 0644, tar.TypeReg, 100, 0, nil},
	}

	for _, file := range compareFiles {
		path := filepath.Join(dir, file.Name)

		// Change the time to something known with nanosec != 0.
		chtime := time.Unix(file.Sec, 987654321)
		if err := os.Chtimes(path, chtime, chtime); err != nil {
			t.Fatal(err)
		}
	}

	// Walk the current state.
	old, err := Walk(dir, nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := makeTarStream(compareFiles)
	if err != nil {
		t.Fatal(err)
	}

	str := NewTarStreamer(bytes.NewBuffer(ts), nil, append(DefaultTarKeywords, "sha1"))
	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}

	new, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}

	// Compare.
	diffs, err := Compare(old, new, append(DefaultTarKeywords, "sha1"))
	if err != nil {
		t.Fatal(err)
	}

	// 0 objects
	if len(diffs) != 0 {
		actualFailure := false
		for i, delta := range diffs {
			// XXX: Tar generation is slightly broken, so we need to ignore some bugs.
			if delta.Path() == "." && delta.Type() == Modified {
				// FIXME: This is a known bug.
				t.Logf("'.' is different in the tar -- this is a bug in the tar generation")

				// The tar generation bug means that '.' is missing a bunch of keys.
				allMissing := true
				for _, keyDelta := range delta.Diff() {
					if keyDelta.Type() != Missing {
						allMissing = false
					}
				}
				if !allMissing {
					t.Errorf("'.' has changed in a way not consistent with known bugs")
				}

				continue
			}

			// XXX: Another bug.
			keys := delta.Diff()
			if len(keys) == 1 && keys[0].Name() == "size" && keys[0].Type() == Missing {
				// FIXME: Also a known bug with tar generation dropping size=.
				t.Logf("'%s' is missing a size= keyword -- a bug in tar generation", delta.Path())

				continue
			}

			actualFailure = true
			buf, err := json.MarshalIndent(delta, "", "  ")
			if err == nil {
				t.Logf("FAILURE: diff[%d] = %s", i, string(buf))
			} else {
				t.Logf("FAILURE: diff[%d] = %#v", i, delta)
			}
		}

		if actualFailure {
			t.Errorf("expected the diff length to be 0, got %d", len(diffs))
		}
	}
}
