package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestUnmarshalBindsYamlAndJSON(t *testing.T) {
	assert := assert.New(t)
	tables := []struct {
		desc     string
		yblob    string
		jblob    string
		expected Binds
		errstr   string
	}{
		{desc: "proper array of source/dest bind allowed",
			yblob: "- source: src1\n  dest: dest1\n",
			jblob: `[{"source": "src1", "dest": "dest1"}]`,
			expected: Binds{
				Bind{Source: "src1", Dest: "dest1"},
			}},
		{desc: "array of bind ascii art",
			yblob: "- src1 -> dest1\n- src2 -> dest2",
			jblob: `["src1 -> dest1", "src2 -> dest2"]`,
			expected: Binds{
				Bind{Source: "src1", Dest: "dest1"},
				Bind{Source: "src2", Dest: "dest2"},
			}},
		{desc: "example mixed valid ascii art and dict",
			yblob: "- src1 -> dest1\n- source: src2\n  dest: dest2\n",
			jblob: `["src1 -> dest1", {"source": "src2", "dest": "dest2"}]`,
			expected: Binds{
				Bind{Source: "src1", Dest: "dest1"},
				Bind{Source: "src2", Dest: "dest2"},
			}},
		// golang encoding/json is case insensitive
		{desc: "capital Source/Dest is not allowed as yaml",
			yblob:    "- Source: src1\n  Dest: dest1\n",
			expected: Binds{},
			errstr:   "xpected 'bind'"},
		{desc: "source is required",
			yblob:    "- Dest: dest1\n",
			jblob:    `[{"Dest": "dest1"}]`,
			expected: Binds{},
			errstr:   "xpected 'bind'"},
		{desc: "must be an array",
			yblob:    "source: src1\ndest: dest1\n",
			jblob:    `{"source": "src1", "dest": "dest1"}`,
			expected: Binds{},
			errstr:   "unmarshal"},
	}
	var err error
	found := Binds{}
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

	for _, t := range tables {
		if t.jblob == "" {
			continue
		}
		err = json.Unmarshal([]byte(t.jblob), &found)
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
