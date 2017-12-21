package mtree

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestWalk(t *testing.T) {
	dh, err := Walk(".", nil, append(DefaultKeywords, "sha1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	numEntries := countTypes(dh)

	fh, err := ioutil.TempFile("", "walk.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fh.Name())
	defer fh.Close()

	if _, err = dh.WriteTo(fh); err != nil {
		t.Fatal(err)
	}
	if _, err := fh.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if dh, err = ParseSpec(fh); err != nil {
		t.Fatal(err)
	}
	for k, v := range countTypes(dh) {
		if numEntries[k] != v {
			t.Errorf("for type %s: expected %d, got %d", k, numEntries[k], v)
		}
	}
}

func TestWalkDirectory(t *testing.T) {
	dh, err := Walk(".", []ExcludeFunc{ExcludeNonDirectories}, []Keyword{"type"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := range dh.Entries {
		for _, keyval := range dh.Entries[i].AllKeys() {
			if dh.Entries[i].Type == FullType || dh.Entries[i].Type == RelativeType {
				if keyval.Keyword() == "type" && keyval.Value() != "dir" {
					t.Errorf("expected only directories, but %q is a %q", dh.Entries[i].Name, keyval.Value())
				}
			}
		}
	}
}
