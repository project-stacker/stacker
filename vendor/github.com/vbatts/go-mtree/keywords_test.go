package mtree

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestKeyValRoundtrip(t *testing.T) {
	kv := KeyVal("xattr.security.selinux=dW5jb25maW5lZF91Om9iamVjdF9yOnVzZXJfaG9tZV90OnMwAA==")
	expected := "xattr.security.selinux"
	got := string(kv.Keyword())
	if got != expected {
		t.Errorf("expected %q; got %q", expected, got)
	}

	expected = "xattr"
	got = string(kv.Keyword().Prefix())
	if got != expected {
		t.Errorf("expected %q; got %q", expected, got)
	}

	expected = "security.selinux"
	got = kv.Keyword().Suffix()
	if got != expected {
		t.Errorf("expected %q; got %q", expected, got)
	}

	expected = "dW5jb25maW5lZF91Om9iamVjdF9yOnVzZXJfaG9tZV90OnMwAA=="
	got = kv.Value()
	if got != expected {
		t.Errorf("expected %q; got %q", expected, got)
	}

	expected = "xattr.security.selinux=farts"
	got = string(kv.NewValue("farts"))
	if got != expected {
		t.Errorf("expected %q; got %q", expected, got)
	}

	expected = "xattr.security.selinux=farts"
	kv1 := KeyVal(got)
	kv2 := kv.NewValue("farts")
	if !kv2.Equal(kv1) {
		t.Errorf("expected equality of %q and %q", kv1, kv2)
	}

}

type fakeFileInfo struct {
	mtime time.Time
}

func (ffi fakeFileInfo) Name() string {
	// noop
	return ""
}

func (ffi fakeFileInfo) Size() int64 {
	// noop
	return -1
}

func (ffi fakeFileInfo) Mode() os.FileMode {
	// noop
	return 0
}

func (ffi fakeFileInfo) ModTime() time.Time {
	return ffi.mtime
}

func (ffi fakeFileInfo) IsDir() bool {
	return ffi.Mode().IsDir()
}

func (ffi fakeFileInfo) Sys() interface{} {
	// noop
	return nil
}

func TestKeywordsTimeNano(t *testing.T) {
	// We have to make sure that timeKeywordFunc always returns the correct
	// formatting with regards to the nanotime.

	for _, test := range []struct {
		sec, nsec int64
	}{
		{1234, 123456789},
		{5555, 987654321},
		{1337, 100000000},
		{8888, 999999999},
		{144123582122, 1},
		{857125628319, 0},
	} {
		mtime := time.Unix(test.sec, test.nsec)
		expected := KeyVal(fmt.Sprintf("time=%d.%9.9d", test.sec, test.nsec))
		got, err := timeKeywordFunc("", fakeFileInfo{
			mtime: mtime,
		}, nil)
		if err != nil {
			t.Errorf("unexpected error while parsing '%q': %q", mtime, err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 KeyVal, but got %d", len(got))
		}
		if expected != got[0] {
			t.Errorf("keyword didn't match, expected '%s' got '%s'", expected, got[0])
		}
	}
}

func TestKeywordsTimeTar(t *testing.T) {
	// tartimeKeywordFunc always has nsec = 0.

	for _, test := range []struct {
		sec, nsec int64
	}{
		{1234, 123456789},
		{5555, 987654321},
		{1337, 100000000},
		{8888, 999999999},
		{144123582122, 1},
		{857125628319, 0},
	} {
		mtime := time.Unix(test.sec, test.nsec)
		expected := KeyVal(fmt.Sprintf("tar_time=%d.%9.9d", test.sec, 0))
		got, err := tartimeKeywordFunc("", fakeFileInfo{
			mtime: mtime,
		}, nil)
		if err != nil {
			t.Errorf("unexpected error while parsing '%q': %q", mtime, err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 KeyVal, but got %d", len(got))
		}
		if expected != got[0] {
			t.Errorf("keyword didn't match, expected '%s' got '%s'", expected, got[0])
		}
	}
}

func TestKeywordSynonym(t *testing.T) {
	checklist := []struct {
		give   string
		expect Keyword
	}{
		{give: "time", expect: "time"},
		{give: "md5", expect: "md5digest"},
		{give: "md5digest", expect: "md5digest"},
		{give: "rmd160", expect: "ripemd160digest"},
		{give: "rmd160digest", expect: "ripemd160digest"},
		{give: "ripemd160digest", expect: "ripemd160digest"},
		{give: "sha1", expect: "sha1digest"},
		{give: "sha1digest", expect: "sha1digest"},
		{give: "sha256", expect: "sha256digest"},
		{give: "sha256digest", expect: "sha256digest"},
		{give: "sha384", expect: "sha384digest"},
		{give: "sha384digest", expect: "sha384digest"},
		{give: "sha512", expect: "sha512digest"},
		{give: "sha512digest", expect: "sha512digest"},
		{give: "xattr", expect: "xattr"},
		{give: "xattrs", expect: "xattr"},
	}

	for i, check := range checklist {
		got := KeywordSynonym(check.give)
		if got != check.expect {
			t.Errorf("%d: expected %q; got %q", i, check.expect, got)
		}
	}
}
