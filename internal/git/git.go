package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var commitHashRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// Export clones the repo (shallow) and removes .git, producing a pure file snapshot.
// ref can be a branch name, tag, or commit hash.
// Returns the resolved commit hash before removing .git.
func Export(repo, ref, dir string) (string, error) {
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
	}

	isCommit := commitHashRe.MatchString(ref)

	if isCommit {
		if out, err := exec.Command("git", "clone", repo, dir).CombinedOutput(); err != nil {
			return "", fmt.Errorf("%s: %s", err, string(out))
		}
		if out, err := exec.Command("git", "-C", dir, "checkout", ref).CombinedOutput(); err != nil {
			return "", fmt.Errorf("checkout %s: %s: %s", ref, err, string(out))
		}
	} else {
		args := []string{"clone", "--depth", "1"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, repo, dir)

		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("%s: %s", err, string(out))
		}
	}

	// Get the commit hash before removing .git
	commit, err := resolveCommit(dir)
	if err != nil {
		commit = "unknown"
	}

	// Remove .git so the cache is a clean file snapshot
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		return commit, err
	}
	return commit, nil
}

// resolveCommit returns the HEAD commit hash of a git repo.
func resolveCommit(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoteHeadCommit returns the latest commit hash of a remote ref without cloning.
func RemoteHeadCommit(repo, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	out, err := exec.Command("git", "ls-remote", repo, ref).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}
	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return "", fmt.Errorf("no ref found for %s", ref)
	}
	return parts[0], nil
}
