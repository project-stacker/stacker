package stacker

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func staleIndexRepo(t *testing.T) (repo string, index []byte) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	repo = t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	git("init", "-q")
	payload := filepath.Join(repo, "payload")
	if err := os.WriteFile(payload, []byte("payload\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "payload")
	git("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-qm", "init")

	// mtime stands in for the uid/gid mismatch seen in a user namespace; both are cached stat fields.
	if err := os.Chtimes(payload, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	return repo, readIndex(t, repo)
}

func readIndex(t *testing.T, repo string) []byte {
	t.Helper()
	index, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestGitStatusRefreshesStaleIndex(t *testing.T) {
	repo, before := staleIndexRepo(t)

	if out, err := exec.Command("git", "-C", repo, "status", "--porcelain", "--untracked-files=no").CombinedOutput(); err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}

	if bytes.Equal(before, readIndex(t, repo)) {
		t.Fatal("git status did not refresh the stale index")
	}
}

func TestGitVersionPreservesIndex(t *testing.T) {
	repo, before := staleIndexRepo(t)

	version, err := GitVersion(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(version, "-dirty") {
		t.Errorf("GitVersion() = %q, should have been clean", version)
	}

	if !bytes.Equal(before, readIndex(t, repo)) {
		t.Error("GitVersion() modified the git index")
	}
}
