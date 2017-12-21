package equalfile

import (
	"bytes"
	//"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

const defaultMaxSize = 10000000000 // Only the first 10^10 bytes are compared.
const defaultBufSize = 20000

type Options struct {
	Debug         bool // enable debugging to stdout
	ForceFileRead bool // prevent shortcut at filesystem level (link, pathname, etc)
	MaxSize       int64
}

type Cmp struct {
	Opt Options

	readCount int
	readMin   int
	readMax   int
	readSum   int64

	hashType         hash.Hash
	hashMatchCompare bool
	hashTable        map[string]hashSum

	buf []byte
}

type hashSum struct {
	result []byte
	err    error
}

// New creates Cmp for multiple comparison mode.
func NewMultiple(buf []byte, options Options, h hash.Hash, compareOnMatch bool) *Cmp {
	c := &Cmp{
		Opt:              options,
		hashType:         h,
		hashMatchCompare: compareOnMatch,
		hashTable:        map[string]hashSum{},
		buf:              buf,
	}
	if c.Opt.MaxSize == 0 {
		c.Opt.MaxSize = defaultMaxSize
	}
	if c.buf == nil || len(c.buf) == 0 {
		c.buf = make([]byte, defaultBufSize)
	}
	return c
}

// New creates Cmp for single comparison mode.
func New(buf []byte, options Options) *Cmp {
	return NewMultiple(buf, options, nil, true)
}

func (c *Cmp) getHash(path string) ([]byte, error) {
	h, found := c.hashTable[path]
	if found {
		return h.result, h.err
	}

	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, openErr
	}
	defer f.Close()

	sum := make([]byte, c.hashType.Size())
	c.hashType.Reset()
	n, copyErr := io.CopyN(c.hashType, f, c.Opt.MaxSize)
	copy(sum, c.hashType.Sum(nil))

	if copyErr == io.EOF && n < c.Opt.MaxSize {
		copyErr = nil
	}

	return c.newHash(path, sum, copyErr)
}

func (c *Cmp) newHash(path string, sum []byte, e error) ([]byte, error) {

	c.hashTable[path] = hashSum{sum, e}

	if c.Opt.Debug {
		fmt.Printf("newHash[%s]=%v: error=[%v]\n", path, hex.EncodeToString(sum), e)
	}

	return sum, e
}

func (c *Cmp) multipleMode() bool {
	return c.hashType != nil
}

// CompareFile verifies that files with names path1, path2 have same contents.
func (c *Cmp) CompareFile(path1, path2 string) (bool, error) {

	if c.multipleMode() {
		h1, err1 := c.getHash(path1)
		if err1 != nil {
			return false, err1
		}
		h2, err2 := c.getHash(path2)
		if err2 != nil {
			return false, err2
		}
		if !bytes.Equal(h1, h2) {
			return false, nil // hashes mismatch
		}
		// hashes match
		if !c.hashMatchCompare {
			return true, nil // accept hash match without byte-by-byte comparison
		}
		// do byte-by-byte comparison
		if c.Opt.Debug {
			fmt.Printf("CompareFile(%s,%s): hash match, will compare bytes\n", path1, path2)
		}
	}

	r1, openErr1 := os.Open(path1)
	if openErr1 != nil {
		return false, openErr1
	}
	defer r1.Close()
	info1, statErr1 := r1.Stat()
	if statErr1 != nil {
		return false, statErr1
	}

	r2, openErr2 := os.Open(path2)
	if openErr2 != nil {
		return false, openErr2
	}
	defer r2.Close()
	info2, statErr2 := r2.Stat()
	if statErr2 != nil {
		return false, statErr2
	}

	if info1.Size() != info2.Size() {
		return false, nil
	}

	if !c.Opt.ForceFileRead {
		// shortcut: ask the filesystem: are these files the same? (link, pathname, etc)
		if os.SameFile(info1, info2) {
			return true, nil
		}
	}

	return c.CompareReader(r1, r2)
}

func (c *Cmp) read(r io.Reader, buf []byte) (int, error) {
	n, err := r.Read(buf)

	if c.Opt.Debug {
		c.readCount++
		c.readSum += int64(n)
		if n < c.readMin {
			c.readMin = n
		}
		if n > c.readMax {
			c.readMax = n
		}
	}

	return n, err
}

// CompareReader verifies that two readers provide same content.
func (c *Cmp) CompareReader(r1, r2 io.Reader) (bool, error) {

	if c.Opt.Debug {
		c.readCount = 0
		c.readMin = 2000000000
		c.readMax = 0
		c.readSum = 0
	}

	equal, err := c.compareReader(r1, r2)

	if c.Opt.Debug {
		fmt.Printf("DEBUG CompareReader(%d,%d): readCount=%d readMin=%d readMax=%d readSum=%d\n",
			len(c.buf), c.Opt.MaxSize, c.readCount, c.readMin, c.readMax, c.readSum)
	}

	return equal, err
}

func (c *Cmp) compareReader(r1, r2 io.Reader) (bool, error) {

	maxSize := c.Opt.MaxSize
	if maxSize < 1 {
		return false, fmt.Errorf("nonpositive max size")
	}

	buf := c.buf

	size := len(buf) / 2
	if size < 1 {
		return false, fmt.Errorf("insufficient buffer size")
	}

	buf1 := buf[:size]
	buf2 := buf[size:]
	eof1 := false
	eof2 := false
	var readSize int64

	for !eof1 && !eof2 {
		n1, err1 := c.read(r1, buf1)
		switch err1 {
		case io.EOF:
			eof1 = true
		case nil:
		default:
			return false, err1
		}

		n2, err2 := c.read(r2, buf2)
		switch err2 {
		case io.EOF:
			eof2 = true
		case nil:
		default:
			return false, err2
		}

		if n1 != n2 {
			return false, fmt.Errorf("compareReader: internal failure: readers returned different sizes")
		}

		if !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil
		}

		readSize += int64(n1)
		if readSize > maxSize {
			return true, fmt.Errorf("max read size reached")
		}
	}

	if !eof1 || !eof2 {
		return false, nil
	}

	return true, nil
}
