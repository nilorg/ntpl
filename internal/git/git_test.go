package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initBareRepo creates a bare git repo with one commit and returns its path.
func initBareRepo(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	bare := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %s", args, err, out)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(work, "hello.txt"), []byte("hello"), 0644)
	run("add", ".")
	run("commit", "-m", "init")
	// Clone to bare repo
	cmd := exec.Command("git", "clone", "--bare", work, bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %s: %s", err, out)
	}
	return bare
}

func TestExport_Branch(t *testing.T) {
	bare := initBareRepo(t)
	dst := filepath.Join(t.TempDir(), "export")

	commit, err := Export(bare, "main", dst)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !commitHashRe.MatchString(commit) {
		t.Errorf("commit should be a hash, got %q", commit)
	}

	// .git should be removed
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error(".git should be removed after Export")
	}

	// File should exist
	data, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read hello.txt: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("hello.txt: got %q", string(data))
	}
}

func TestExport_EmptyRef(t *testing.T) {
	bare := initBareRepo(t)
	dst := filepath.Join(t.TempDir(), "export")

	commit, err := Export(bare, "", dst)
	if err != nil {
		t.Fatalf("Export with empty ref: %v", err)
	}
	if commit == "" || commit == "unknown" {
		t.Errorf("commit should be resolved, got %q", commit)
	}
}

func TestExport_CommitHash(t *testing.T) {
	bare := initBareRepo(t)

	// Get the commit hash
	out, err := exec.Command("git", "-C", bare, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	hash := strings.TrimSpace(string(out))

	dst := filepath.Join(t.TempDir(), "export")
	commit, err := Export(bare, hash, dst)
	if err != nil {
		t.Fatalf("Export by commit: %v", err)
	}
	if commit != hash {
		t.Errorf("commit: got %q, want %q", commit, hash)
	}
}

func TestExport_OverwriteExisting(t *testing.T) {
	bare := initBareRepo(t)
	dst := filepath.Join(t.TempDir(), "export")

	os.MkdirAll(dst, 0755)
	os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("stale"), 0644)

	if _, err := Export(bare, "main", dst); err != nil {
		t.Fatalf("Export overwrite: %v", err)
	}

	// stale.txt should be gone, hello.txt should exist
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale.txt should be removed")
	}
	if _, err := os.Stat(filepath.Join(dst, "hello.txt")); err != nil {
		t.Error("hello.txt should exist")
	}
}

func TestExport_InvalidRepo(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "export")
	_, err := Export("/nonexistent/repo.git", "main", dst)
	if err == nil {
		t.Fatal("expected error for invalid repo")
	}
}

func TestResolveCommit(t *testing.T) {
	work := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %s", args, err, out)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(work, "f.txt"), []byte("x"), 0644)
	run("add", ".")
	run("commit", "-m", "init")

	commit, err := resolveCommit(work)
	if err != nil {
		t.Fatalf("resolveCommit: %v", err)
	}
	if !commitHashRe.MatchString(commit) {
		t.Errorf("not a valid hash: %q", commit)
	}
}

func TestResolveCommit_InvalidDir(t *testing.T) {
	_, err := resolveCommit("/nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid dir")
	}
}

func TestRemoteHeadCommit(t *testing.T) {
	bare := initBareRepo(t)

	commit, err := RemoteHeadCommit(bare, "main")
	if err != nil {
		t.Fatalf("RemoteHeadCommit: %v", err)
	}
	if !commitHashRe.MatchString(commit) {
		t.Errorf("not a valid hash: %q", commit)
	}
}

func TestRemoteHeadCommit_EmptyRef(t *testing.T) {
	bare := initBareRepo(t)

	commit, err := RemoteHeadCommit(bare, "")
	if err != nil {
		t.Fatalf("RemoteHeadCommit empty ref: %v", err)
	}
	if !commitHashRe.MatchString(commit) {
		t.Errorf("not a valid hash: %q", commit)
	}
}

func TestRemoteHeadCommit_InvalidRepo(t *testing.T) {
	_, err := RemoteHeadCommit("/nonexistent/repo.git", "main")
	if err == nil {
		t.Fatal("expected error for invalid repo")
	}
}

func TestCommitHashRe(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Valid commit hashes
		{"abcdef1", true},
		{"abc1234", true},
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", true}, // full 40 chars
		{"1234567890abcdef1234567890abcdef12345678", true}, // full 40 chars
		{"abcdef1234567", true},                            // 13 chars

		// Invalid: too short
		{"abc123", false},
		{"abcdef", false},

		// Invalid: uppercase
		{"ABCDEF1", false},

		// Invalid: non-hex chars
		{"abcdefg", false},
		{"main", false},
		{"v1.0.0", false},
		{"develop", false},

		// Invalid: empty
		{"", false},

		// Invalid: too long
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2a", false}, // 41 chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := commitHashRe.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("commitHashRe.Match(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
