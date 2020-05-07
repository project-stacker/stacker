package stacker

import (
	"os/exec"
	"strings"

	"github.com/apex/log"
	"github.com/pkg/errors"
)

// gitHash generates a version string similar to git describe --always
func gitHash(path string, short bool) (string, error) {

	// Get hash
	args := []string{"-C", path, "rev-parse", "HEAD"}
	if short {
		args = []string{"-C", path, "rev-parse", "--short", "HEAD"}
	}
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GitVersion generates a version string similar to what git describe --always
// does, with -dirty on the end if the git repo had local changes.
func GitVersion(path string) (string, error) {

	var vers string
	// Obtain commit hash
	args := []string{"-C", path, "describe", "--tags"}
	output, err := exec.Command("git", args...).CombinedOutput()
	if err == nil {
		vers = strings.TrimSpace(string(output))
	} else {
		log.Debug("'git describe --tags' failed, falling back to hash")
		vers, err = gitHash(path, false)
		if err != nil {
			return "", err
		}
	}

	// Check if there are local changes
	args = []string{"-C", path, "status", "--porcelain", "--untracked-files=no"}
	output, err = exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", err
	}

	if len(output) == 0 {
		// Commit is clean, no local changes found
		return vers, nil
	}

	return vers + "-dirty", nil
}

// NewGitLayerTag version generates a commit-<id> tag to be used for uploading an image to a docker registry
func NewGitLayerTag(path string) (string, error) {

	// Check if there are local changes
	args := []string{"-C", path, "status", "--porcelain", "--untracked-files=no"}
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", err
	}

	// If there are local changes, we don't generate a git commit tag for the new layer
	if len(output) != 0 {
		return "", errors.Errorf("commit is dirty so don't generate a tag based on git commit: %s", output)
	}

	// Determine git hash
	hash, err := gitHash(path, true)
	if err != nil {
		return "", err
	}

	// Add commit id in tag
	return "commit-" + hash, nil
}
