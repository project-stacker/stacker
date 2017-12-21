package mtree

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/vbatts/go-mtree/xattr"
)

func init() {
	//logrus.SetLevel(logrus.DebugLevel)
}

func TestXattrUpdate(t *testing.T) {
	content := []byte("I know half of you half as well as I ought to")
	// a bit dirty to create/destory a directory in cwd, but often /tmp is
	// mounted tmpfs and doesn't support xattrs
	dir, err := ioutil.TempDir(".", "test.xattr.restore.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		t.Fatal(err)
	}

	if err := xattr.Set(dir, "user.test", []byte("directory")); err != nil {
		t.Skip(fmt.Sprintf("skipping: %q does not support xattrs", dir))
	}
	if err := xattr.Set(tmpfn, "user.test", []byte("regular file")); err != nil {
		t.Fatal(err)
	}

	// Walk this tempdir
	dh, err := Walk(dir, nil, append(DefaultKeywords, []Keyword{"xattr", "sha1"}...), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Now check that we're sane
	res, err := Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expecting no failures, but got %q", res)
	}

	if err := xattr.Set(tmpfn, "user.test", []byte("let it fly")); err != nil {
		t.Fatal(err)
	}

	// Now check that we fail the check
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Error("expected failures (like xattrs), but got none")
	}

	// restore the xattrs to original
	res, err = Update(dir, dh, append(DefaultUpdateKeywords, "xattr"), nil)
	if err != nil {
		t.Error(err)
	}
	if len(res) != 0 {
		t.Errorf("expecting no failures, but got %q", res)
	}

	// Now check that we're sane again
	res, err = Check(dir, dh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		// pretty this shit up
		buf, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			t.Errorf("expecting no failures, but got %q", res)
		} else {
			t.Errorf("expecting no failures, but got %s", string(buf))
		}
	}

	// TODO make a test for xattr here. Likely in the user space for privileges. Even still this may be prone to error for some tmpfs don't act right with xattrs. :-\
	// I'd hate to have to t.Skip() a test rather than fail alltogether.
}
