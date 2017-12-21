package mtree

import (
	"io/ioutil"
	"os"
	"testing"
)

var (
	testFiles = []struct {
		Name   string
		Counts map[EntryType]int
		Len    int64
	}{
		{
			Name: "testdata/source.mtree",
			Counts: map[EntryType]int{
				FullType:     0,
				RelativeType: 45,
				CommentType:  37,
				SpecialType:  7,
				DotDotType:   17,
				BlankType:    34,
			},
			Len: int64(7887),
		},
		{
			Name: "testdata/source.casync-mtree",
			Counts: map[EntryType]int{
				FullType:     744,
				RelativeType: 56,
				CommentType:  37,
				SpecialType:  7,
				DotDotType:   17,
				BlankType:    34,
			},
			Len: int64(168439),
		},
	}
)

func TestParser(t *testing.T) {
	for i, tf := range testFiles {
		_ = i
		func() {
			fh, err := os.Open(tf.Name)
			if err != nil {
				t.Error(err)
				return
			}
			defer fh.Close()

			dh, err := ParseSpec(fh)
			if err != nil {
				t.Error(err)
			}
			/*
				if i == 1 {
					buf, err := xml.MarshalIndent(dh, "", "  ")
					if err == nil {
						t.Error(string(buf))
					}
				}
			*/
			gotNums := countTypes(dh)
			for typ, num := range tf.Counts {
				if gNum, ok := gotNums[typ]; ok {
					if num != gNum {
						t.Errorf("for type %s: expected %d, got %d", typ, num, gNum)
					}
				}
			}

			i, err := dh.WriteTo(ioutil.Discard)
			if err != nil {
				t.Error(err)
			}
			if i != tf.Len {
				t.Errorf("expected to write %d, but wrote %d", tf.Len, i)
			}

		}()
	}
}

func countTypes(dh *DirectoryHierarchy) map[EntryType]int {
	nT := map[EntryType]int{}
	for i := range dh.Entries {
		typ := dh.Entries[i].Type
		if _, ok := nT[typ]; !ok {
			nT[typ] = 1
		} else {
			nT[typ]++
		}
	}
	return nT
}
