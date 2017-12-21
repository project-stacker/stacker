// +build go1.7

package mtree

import (
	"container/heap"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func TestUpdate(t *testing.T) {
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

	// Touch a file, so the mtime changes.
	now := time.Now()
	if err := os.Chtimes(tmpfn, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(tmpfn, os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	// Changing user is a little tough, but the group can be changed by a limited user to any group that the user is a member of. So just choose one that is not the current main group.
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	ugroups, err := u.GroupIds()
	if err != nil {
		t.Fatal(err)
	}
	for _, ugroup := range ugroups {
		if ugroup == u.Gid {
			continue
		}
		gid, err := strconv.Atoi(ugroup)
		if err != nil {
			t.Fatal(ugroup)
		}
		if err := os.Lchown(tmpfn, -1, gid); err != nil {
			t.Fatal(err)
		}
	}

	// Check for sanity. This ought to have failures
	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Error("expected failures (like mtimes), but got none")
	}
	//dh.WriteTo(os.Stdout)

	res, err = Update(dir, dh, DefaultUpdateKeywords, nil)
	if err != nil {
		t.Error(err)
	}
	if len(res) > 0 {
		// pretty this shit up
		buf, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			t.Errorf("%#v", res)
		}
		t.Error(string(buf))
	}

	// Now check that we're sane again
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// should have no failures now
	if len(res) > 0 {
		// pretty this shit up
		buf, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			t.Errorf("%#v", res)
		} else {
			t.Error(string(buf))
		}
	}

}

func TestPathUpdateHeap(t *testing.T) {
	h := &pathUpdateHeap{
		pathUpdate{Path: "not/the/longest"},
		pathUpdate{Path: "almost/the/longest"},
		pathUpdate{Path: "."},
		pathUpdate{Path: "short"},
	}
	heap.Init(h)
	v := "this/is/one/is/def/the/longest"
	heap.Push(h, pathUpdate{Path: v})

	longest := len(v)
	var p string
	for h.Len() > 0 {
		p = heap.Pop(h).(pathUpdate).Path
		if len(p) > longest {
			t.Errorf("expected next path to be shorter, but it was not %q is longer than %d", p, longest)
		}
	}
	if p != "." {
		t.Errorf("expected \".\" to be the last, but got %q", p)
	}
}
