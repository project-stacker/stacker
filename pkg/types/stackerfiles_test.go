package types

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// writeStackerfile writes a stackerfile with a single layer named after the
// file and the given prerequisites, and returns its path.
func writeStackerfile(t *testing.T, dir, name string, prerequisites ...string) string {
	content := "config:\n  prerequisites:\n"
	for _, prereq := range prerequisites {
		content += fmt.Sprintf("    - %s\n", prereq)
	}
	content += fmt.Sprintf("%s:\n  from:\n    type: scratch\n", name)

	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("couldn't write stackerfile: %s", err)
	}

	return path
}

func TestNewStackerFilesPrerequisites(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	// c depends on a and b, and b also depends on a
	a := writeStackerfile(t, dir, "a")
	b := writeStackerfile(t, dir, "b", "a.yaml")
	c := writeStackerfile(t, dir, "c", "a.yaml", "b.yaml")

	sfm, err := NewStackerFiles([]string{c}, false, nil)
	if !assert.NoError(err) {
		return
	}

	assert.Len(sfm, 3)
	for _, path := range []string{a, b, c} {
		assert.Contains(sfm, path)
	}
}

func TestNewStackerFilesPrerequisiteCycle(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	a := writeStackerfile(t, dir, "a", "b.yaml")
	b := writeStackerfile(t, dir, "b", "a.yaml")
	self := writeStackerfile(t, dir, "self", "self.yaml")

	_, err := NewStackerFiles([]string{a}, false, nil)
	assert.EqualError(err, fmt.Sprintf("prerequisite cycle detected: %s -> %s -> %s", a, b, a))

	_, err = NewStackerFiles([]string{self}, false, nil)
	assert.EqualError(err, fmt.Sprintf("prerequisite cycle detected: %s -> %s", self, self))
}
