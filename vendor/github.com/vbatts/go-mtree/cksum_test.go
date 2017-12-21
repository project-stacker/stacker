package mtree

import (
	"os"
	"testing"
)

var (
	checkFile        = "./testdata/source.mtree"
	checkSum  uint32 = 1048442895
	checkSize        = 9110
)

// testing that the cksum function matches that of cksum(1) utility (silly POSIX crc32)
func TestCksum(t *testing.T) {
	fh, err := os.Open(checkFile)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sum, i, err := cksum(fh)
	if err != nil {
		t.Fatal(err)
	}
	if i != checkSize {
		t.Errorf("%q: expected size %d, got %d", checkFile, checkSize, i)
	}
	if sum != checkSum {
		t.Errorf("%q: expected sum %d, got %d", checkFile, checkSum, sum)
	}
}
