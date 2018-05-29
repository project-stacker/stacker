package stacker

import (
	"os/exec"
	"strings"
)

// Git version generates a version string similar to what git describe --always
// does, with -dirty on the end if the git repo had local changes.
func GitVersion(path string) (string, error) {
	output, err := exec.Command("git", "-C", path, "status", "--porcelain", "--untracked-files=no").CombinedOutput()
	if err != nil {
		return "", err
	}

	isClean := len(output) == 0

	output, err = exec.Command("git", "-C", path, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", err
	}

	hash := strings.TrimSpace(string(output))

	if isClean {
		return hash, nil
	}

	return hash + "-dirty", nil
}
