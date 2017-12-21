// +build !windows

package equalfile

import (
	"crypto/sha256"
	"testing"
)

func TestCompareLimitBroken(t *testing.T) {
	buf := make([]byte, 1000)
	compare(t, New(nil, Options{ForceFileRead: true, MaxSize: -1}), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, New(buf, Options{ForceFileRead: true, MaxSize: 0}), "/etc/passwd", "/etc/passwd", expectEqual) // will switch to 10G default limit
	compare(t, New(buf, Options{ForceFileRead: true, MaxSize: 1}), "/etc/passwd", "/etc/passwd", expectError) // will reach 1-byte limit
	compare(t, New(buf, Options{ForceFileRead: true, MaxSize: 1000000}), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufBroken(t *testing.T) {
	var limit int64 = 1000000
	options := Options{ForceFileRead: true, MaxSize: limit}
	compare(t, New(nil, options), "/etc/passwd", "/etc/passwd", expectEqual)             // will switch to 20K default buf
	compare(t, New([]byte{}, options), "/etc/passwd", "/etc/passwd", expectEqual)        // will switch to 20K default buf
	compare(t, New(make([]byte, 0), options), "/etc/passwd", "/etc/passwd", expectEqual) // will switch to 20K default buf
	compare(t, New(make([]byte, 1), options), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, New(make([]byte, 2), options), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufSmall(t *testing.T) {
	batch(t, 100000, make([]byte, 100))
}

func TestCompareBufLarge(t *testing.T) {
	batch(t, 100000000, make([]byte, 10000000))
}

func TestCompareLimitBrokenMultiple(t *testing.T) {
	buf := make([]byte, 1000)
	compare(t, NewMultiple(nil, Options{ForceFileRead: true, MaxSize: -1}, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, NewMultiple(buf, Options{ForceFileRead: true, MaxSize: 0}, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual) // will switch to 10G default limit
	compare(t, NewMultiple(buf, Options{ForceFileRead: true, MaxSize: 1}, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectError) // will reach 1-byte limit
	compare(t, NewMultiple(buf, Options{ForceFileRead: true, MaxSize: 1000000}, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufBrokenMultiple(t *testing.T) {
	var limit int64 = 1000000
	options := Options{ForceFileRead: true, MaxSize: limit}
	compare(t, NewMultiple(nil, options, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual)             // will switch to 20K default buf
	compare(t, NewMultiple([]byte{}, options, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual)        // will switch to 20K default buf
	compare(t, NewMultiple(make([]byte, 0), options, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual) // will switch to 20K default buf
	compare(t, NewMultiple(make([]byte, 1), options, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, NewMultiple(make([]byte, 2), options, sha256.New(), true), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufSmallMultiple(t *testing.T) {
	batchMultiple(t, 100000, make([]byte, 100))
}

func TestCompareBufLargeMultiple(t *testing.T) {
	batchMultiple(t, 100000000, make([]byte, 10000000))
}

func batch(t *testing.T, limit int64, buf []byte) {
	c := New(buf, Options{ForceFileRead: true, MaxSize: limit})
	compare(t, c, "/etc", "/etc", expectError)
	compare(t, c, "/etc/ERROR", "/etc/passwd", expectError)
	compare(t, c, "/etc/passwd", "/etc/ERROR", expectError)
	compare(t, c, "/etc/passwd", "/etc/passwd", expectEqual)
	compare(t, c, "/etc/passwd", "/etc/group", expectUnequal)
	compare(t, c, "/dev/null", "/dev/null", expectEqual)
	compare(t, c, "/dev/urandom", "/dev/urandom", expectUnequal)
	compare(t, c, "/dev/zero", "/dev/zero", expectError)
}

func batchMultiple(t *testing.T, limit int64, buf []byte) {
	c := NewMultiple(buf, Options{ForceFileRead: true, MaxSize: limit}, sha256.New(), true)
	compare(t, c, "/etc", "/etc", expectError)
	compare(t, c, "/etc/ERROR", "/etc/passwd", expectError)
	compare(t, c, "/etc/passwd", "/etc/ERROR", expectError)
	compare(t, c, "/etc/passwd", "/etc/passwd", expectEqual)
	compare(t, c, "/etc/passwd", "/etc/group", expectUnequal)
	compare(t, c, "/dev/null", "/dev/null", expectEqual)
	compare(t, c, "/dev/urandom", "/dev/urandom", expectUnequal)
	compare(t, c, "/dev/zero", "/dev/zero", expectError)
}

func compare(t *testing.T, c *Cmp, path1, path2 string, expect int) {
	//t.Logf("compare(%s,%s) limit=%d buf=%d", path1, path2, c.Opt.MaxSize, len(c.buf))
	equal, err := c.CompareFile(path1, path2)
	if err != nil {
		if expect != expectError {
			t.Errorf("compare: unexpected error: CompareFile(%s,%s,%d,%d): %v", path1, path2, c.Opt.MaxSize, len(c.buf), err)
		}
		return
	}
	if equal {
		if expect != expectEqual {
			t.Errorf("compare: unexpected equal: CompareFile(%s,%s,%d,%d)", path1, path2, c.Opt.MaxSize, len(c.buf))
		}
		return
	}
	if expect != expectUnequal {
		t.Errorf("compare: unexpected unequal: CompareFile(%s,%s,%d,%d)", path1, path2, c.Opt.MaxSize, len(c.buf))
	}
}
