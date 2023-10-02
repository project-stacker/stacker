package types

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// the empty/unset Uid/Gid
const eUGid = -1

func modePtr(mode int) *fs.FileMode {
	m := fs.FileMode(mode)
	return &m
}

func TestGetImportFromInterface(t *testing.T) {
	assert := assert.New(t)
	hash1 := "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"
	tables := []struct {
		desc     string
		val      interface{}
		expected Import
		errstr   string
	}{
		{desc: "basic string",
			val:      "/path/to/file",
			expected: Import{Path: "/path/to/file", Uid: eUGid, Gid: eUGid}},
		{desc: "relative string",
			val:      "path/to/file",
			expected: Import{Path: "path/to/file", Uid: eUGid, Gid: eUGid}},
		{desc: "dict no dest",
			val: map[interface{}]interface{}{
				"path": "/path/to/file",
				"hash": hash1,
			},
			expected: Import{Path: "/path/to/file", Dest: "", Hash: hash1, Uid: eUGid, Gid: eUGid}},
		{desc: "dest cannot be relative",
			val: map[interface{}]interface{}{
				"path": "src1",
				"dest": "dest1",
			},
			errstr: "cannot be relative",
		},
		{desc: "uid cannot be negative",
			val: map[interface{}]interface{}{
				"path": "src1",
				"uid":  -2,
			},
			errstr: "cannot be negative",
		},
		{desc: "gid cannot be negative",
			val: map[interface{}]interface{}{
				"path": "src1",
				"gid":  -2,
			},
			errstr: "cannot be negative",
		},
		{desc: "gid must be an int",
			val: map[interface{}]interface{}{
				"path": "src1",
				"gid":  "100",
			},
			errstr: "not an integer",
		},
		{desc: "mode present",
			val: map[interface{}]interface{}{
				"path": "src1",
				"mode": 0755,
			},
			expected: Import{Path: "src1", Dest: "", Mode: modePtr(0755), Uid: eUGid, Gid: eUGid}},
		{desc: "path must be present",
			val: map[interface{}]interface{}{
				"uid":  0,
				"dest": "/path/to/file",
			},
			errstr: "No 'path' entry found",
		},
		{desc: "bad type - list",
			val:    []interface{}{"foo", "bar"},
			errstr: "could not read imports entry",
		},
		{desc: "bad type - non-string-keys",
			val: map[interface{}]interface{}{
				1: "target",
			},
			errstr: "is not a string",
		},
		{desc: "bad type - path",
			val: map[interface{}]interface{}{
				"path": 1111,
			},
			errstr: "is not a string",
		},
	}

	var found Import
	var err error
	for _, t := range tables {
		found, err = getImportFromInterface(t.val)
		if t.errstr == "" {
			assert.NoError(err, t.desc)
			assert.Equal(t.expected, found, t.desc)
		} else {
			assert.ErrorContains(err, t.errstr, t.desc)
		}
	}
}

func TestUnmarshalImports(t *testing.T) {
	assert := assert.New(t)
	tables := []struct {
		desc     string
		yblob    string
		expected Imports
		errstr   string
	}{
		{desc: "import can be a singular string",
			yblob: "f1",
			expected: Imports{
				Import{Path: "f1", Uid: eUGid, Gid: eUGid},
			}},
		{desc: "import might be present and explicit null",
			yblob:    "null",
			expected: nil},
		{desc: "imports should not be a dict",
			yblob:    "path: /path/to/file\ndest: /path/to/dest\n",
			expected: Imports{},
			errstr:   "xpected an array"},
		{desc: "example valid mixed string and dict",
			yblob: "- f1\n- path: f2\n",
			expected: Imports{
				Import{Path: "f1", Uid: eUGid, Gid: eUGid},
				Import{Path: "f2", Uid: eUGid, Gid: eUGid},
			}},
	}
	var err error
	found := Imports{}
	for _, t := range tables {
		err = yaml.Unmarshal([]byte(t.yblob), &found)
		if t.errstr == "" {
			if !assert.NoError(err, t.desc) {
				continue
			}
			assert.Equal(t.expected, found)
		} else {
			assert.ErrorContains(err, t.errstr, t.desc)
		}
	}
}
