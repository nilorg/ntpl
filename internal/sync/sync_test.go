package sync

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilorg/ntpl/internal/config"
)

func TestReplaceVars(t *testing.T) {
	tests := []struct {
		name string
		data string
		vars map[string]string
		want string
	}{
		{
			name: "single var",
			data: "hello {ntpl:name}!",
			vars: map[string]string{"name": "world"},
			want: "hello world!",
		},
		{
			name: "multiple vars",
			data: "org={ntpl:org} project={ntpl:project}",
			vars: map[string]string{"org": "nilorg", "project": "ntpl"},
			want: "org=nilorg project=ntpl",
		},
		{
			name: "undefined var preserved",
			data: "keep {ntpl:unknown} as is",
			vars: map[string]string{"other": "val"},
			want: "keep {ntpl:unknown} as is",
		},
		{
			name: "empty vars",
			data: "no change {ntpl:x}",
			vars: map[string]string{},
			want: "no change {ntpl:x}",
		},
		{
			name: "nil vars",
			data: "no change {ntpl:x}",
			vars: nil,
			want: "no change {ntpl:x}",
		},
		{
			name: "no placeholders",
			data: "plain text",
			vars: map[string]string{"x": "y"},
			want: "plain text",
		},
		{
			name: "var at boundaries",
			data: "{ntpl:start}middle{ntpl:end}",
			vars: map[string]string{"start": "[", "end": "]"},
			want: "[middle]",
		},
		{
			name: "repeated var",
			data: "{ntpl:v}-{ntpl:v}",
			vars: map[string]string{"v": "X"},
			want: "X-X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(replaceVars([]byte(tt.data), tt.vars))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		excludes []string
		want     bool
	}{
		{
			name:     "exact match",
			rel:      "vendor",
			excludes: []string{"vendor"},
			want:     true,
		},
		{
			name:     "glob pattern",
			rel:      "debug.log",
			excludes: []string{"*.log"},
			want:     true,
		},
		{
			name:     "prefix match",
			rel:      "vendor/pkg/file.go",
			excludes: []string{"vendor"},
			want:     true,
		},
		{
			name:     "base name match",
			rel:      "some/deep/path/.ntpl",
			excludes: []string{".ntpl"},
			want:     true,
		},
		{
			name:     "no match",
			rel:      "src/main.go",
			excludes: []string{"vendor", "*.log"},
			want:     false,
		},
		{
			name:     "empty excludes",
			rel:      "anything",
			excludes: nil,
			want:     false,
		},
		{
			name:     ".ntpl.yaml excluded",
			rel:      ".ntpl.yaml",
			excludes: []string{".ntpl.yaml"},
			want:     true,
		},
		{
			name:     "nested glob",
			rel:      "test.tmp",
			excludes: []string{"*.tmp"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsExcluded(tt.rel, tt.excludes)
			if got != tt.want {
				t.Errorf("IsExcluded(%q, %v) = %v, want %v", tt.rel, tt.excludes, got, tt.want)
			}
		})
	}
}

func TestSyncDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source files
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("content A"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("content B"), 0644)
	os.WriteFile(filepath.Join(src, "skip.log"), []byte("skip"), 0644)

	excludes := []string{"*.log"}
	vars := map[string]string{}

	if err := syncDir(src, dst, excludes, vars); err != nil {
		t.Fatalf("syncDir: %v", err)
	}

	// Check synced file
	data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatalf("read a.txt: %v", err)
	}
	if string(data) != "content A" {
		t.Errorf("a.txt: got %q", string(data))
	}

	// Check nested file
	data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("read sub/b.txt: %v", err)
	}
	if string(data) != "content B" {
		t.Errorf("sub/b.txt: got %q", string(data))
	}

	// Check excluded file not synced
	if _, err := os.Stat(filepath.Join(dst, "skip.log")); err == nil {
		t.Error("skip.log should not be synced")
	}
}

func TestSyncDir_WithVarReplace(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "config.yaml"), []byte("name: {ntpl:project}\norg: {ntpl:org}"), 0644)

	vars := map[string]string{"project": "myapp", "org": "myteam"}

	if err := syncDir(src, dst, nil, vars); err != nil {
		t.Fatalf("syncDir: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "name: myapp\norg: myteam" {
		t.Errorf("got %q", string(data))
	}
}

func TestSyncDir_OverwriteExisting(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "file.txt"), []byte("new content"), 0644)
	os.WriteFile(filepath.Join(dst, "file.txt"), []byte("old content"), 0644)

	if err := syncDir(src, dst, nil, nil); err != nil {
		t.Fatalf("syncDir: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(data) != "new content" {
		t.Errorf("file not overwritten: %q", string(data))
	}
}

func TestSyncDir_ExcludeDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, ".ntpl", "sub"), 0755)
	os.WriteFile(filepath.Join(src, ".ntpl", "sub", "f.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(src, "ok.txt"), []byte("ok"), 0644)

	if err := syncDir(src, dst, []string{".ntpl"}, nil); err != nil {
		t.Fatalf("syncDir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".ntpl")); err == nil {
		t.Error(".ntpl dir should be excluded")
	}
	if _, err := os.Stat(filepath.Join(dst, "ok.txt")); err != nil {
		t.Error("ok.txt should exist")
	}
}

func TestDiffDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "same.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(dst, "same.txt"), []byte("content"), 0644)

	os.WriteFile(filepath.Join(src, "changed.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(dst, "changed.txt"), []byte("old"), 0644)

	os.WriteFile(filepath.Join(src, "template_only.txt"), []byte("only in template"), 0644)
	os.WriteFile(filepath.Join(dst, "project_only.txt"), []byte("only in project"), 0644)

	// Capture stdout isn't easy, just ensure no error
	err := diffDir(src, dst, nil, nil)
	if err != nil {
		t.Fatalf("diffDir: %v", err)
	}
}

func TestDiffDir_WithVars(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// After var replacement, these should be identical
	os.WriteFile(filepath.Join(src, "f.txt"), []byte("hello {ntpl:name}"), 0644)
	os.WriteFile(filepath.Join(dst, "f.txt"), []byte("hello world"), 0644)

	err := diffDir(src, dst, nil, map[string]string{"name": "world"})
	if err != nil {
		t.Fatalf("diffDir: %v", err)
	}
	// No explicit assertion needed; just ensuring no crash
}

func TestDryRunDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "new.txt"), []byte("new file"), 0644)
	os.WriteFile(filepath.Join(src, "update.txt"), []byte("updated"), 0644)
	os.WriteFile(filepath.Join(dst, "update.txt"), []byte("original"), 0644)

	// Just verify no panic; output goes to stdout
	dryRunDir(src, dst, nil, nil)

	// Verify dst was NOT changed
	data, _ := os.ReadFile(filepath.Join(dst, "update.txt"))
	if string(data) != "original" {
		t.Error("dryRunDir should not modify files")
	}
	if _, err := os.Stat(filepath.Join(dst, "new.txt")); err == nil {
		t.Error("dryRunDir should not create files")
	}
}

func TestMergeExcludes(t *testing.T) {
	// Need to set up a temp working dir without .ntplignore
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := config.Sync{
		Exclude: []string{"vendor", "dist"},
	}

	excludes := mergeExcludes(cfg)
	// Should contain builtins + user excludes
	found := make(map[string]bool)
	for _, e := range excludes {
		found[e] = true
	}

	for _, b := range config.BuiltinExcludes {
		if !found[b] {
			t.Errorf("missing builtin exclude: %s", b)
		}
	}
	if !found["dist"] {
		t.Errorf("missing user excludes: %v", excludes)
	}
}

func TestMergeExcludes_WithIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	os.WriteFile(".ntplignore", []byte("*.bak\ntmp\n"), 0644)

	cfg := config.Sync{Exclude: []string{"vendor"}}
	excludes := mergeExcludes(cfg)

	found := make(map[string]bool)
	for _, e := range excludes {
		found[e] = true
	}

	if !found["*.bak"] || !found["tmp"] {
		t.Errorf("missing ntplignore excludes: %v", excludes)
	}
}

func TestPrintUnifiedDiff(t *testing.T) {
	// Just verify no panic with various inputs
	printUnifiedDiff("test", []byte("line1\nline2"), []byte("line1\nline3"))
	printUnifiedDiff("test", []byte(""), []byte("new"))
	printUnifiedDiff("test", []byte("old"), []byte(""))
	printUnifiedDiff("test", []byte("same"), []byte("same"))
}

func TestSyncDir_EmptySrc(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := syncDir(src, dst, nil, nil); err != nil {
		t.Fatalf("syncDir empty: %v", err)
	}

	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Errorf("dst should be empty after syncing empty src")
	}
}

func TestSyncDir_PreservesFileContent(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Binary-like content
	content := make([]byte, 256)
	for i := range content {
		content[i] = byte(i)
	}
	os.WriteFile(filepath.Join(src, "binary"), content, 0644)

	if err := syncDir(src, dst, nil, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "binary"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Error("binary content not preserved")
	}
}

func TestTemplateDir(t *testing.T) {
	tpl := config.Template{Name: "base"}
	dir := templateDir(tpl)
	if dir != filepath.Join(".ntpl", "template", "base") {
		t.Errorf("templateDir: got %q", dir)
	}
}

func TestRunHook_Empty(t *testing.T) {
	err := runHook("test", "")
	if err != nil {
		t.Errorf("empty hook should not error: %v", err)
	}
}

func TestRunHook_Success(t *testing.T) {
	err := runHook("test", "true")
	if err != nil {
		t.Errorf("true hook should succeed: %v", err)
	}
}

func TestRunHook_Failure(t *testing.T) {
	err := runHook("test", "false")
	if err == nil {
		t.Error("false hook should fail")
	}
}

func TestRunHook_Script(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hook_ran")
	err := runHook("test", "touch "+marker)
	if err != nil {
		t.Fatalf("hook error: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("hook should have created marker file")
	}
}

func TestLoadRemoteDefaults_NoRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Sync: config.Sync{Include: []string{"src/"}},
		Vars: map[string]string{"k": "v"},
	}

	s, vars := loadRemoteDefaults(cfg, dir)
	if len(s.Include) != 1 || s.Include[0] != "src/" {
		t.Errorf("should return local sync: %+v", s)
	}
	if vars["k"] != "v" {
		t.Errorf("should return local vars: %+v", vars)
	}
}

func TestLoadRemoteDefaults_WithRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	remoteYAML := `sync:
  exclude:
    - "*.generated"
vars:
  remote_key: remote_val
`
	os.WriteFile(filepath.Join(dir, ".ntpl.yaml"), []byte(remoteYAML), 0644)

	cfg := config.Config{
		Vars: map[string]string{"local_key": "local_val"},
	}

	s, vars := loadRemoteDefaults(cfg, dir)
	// Empty local exclude -> should pick up remote
	if len(s.Exclude) != 1 || s.Exclude[0] != "*.generated" {
		t.Errorf("should have remote excludes: %+v", s)
	}
	if vars["remote_key"] != "remote_val" {
		t.Errorf("should have remote var: %+v", vars)
	}
	if vars["local_key"] != "local_val" {
		t.Errorf("should keep local var: %+v", vars)
	}
}

func TestDryRunDir_ExcludeDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, "skip", "deep"), 0755)
	os.WriteFile(filepath.Join(src, "skip", "deep", "f.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644)

	// Just should not panic, and skip dir should be excluded
	dryRunDir(src, dst, []string{"skip"}, nil)
}

func TestDiffDir_Excludes(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dst, "keep.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(src, "skip.log"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dst, "skip.log"), []byte("y"), 0644)

	err := diffDir(src, dst, []string{"*.log"}, nil)
	if err != nil {
		t.Fatalf("diffDir: %v", err)
	}
}

// initBareRepo creates a bare git repo with template files and returns its path.
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
	os.WriteFile(filepath.Join(work, "file.txt"), []byte("template content"), 0644)
	os.WriteFile(filepath.Join(work, "hello.txt"), []byte("hello {ntpl:name}"), 0644)
	run("add", ".")
	run("commit", "-m", "init")

	cmd := exec.Command("git", "clone", "--bare", work, bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %s: %s", err, out)
	}
	return bare
}

func TestRun_DryRun(t *testing.T) {
	bare := initBareRepo(t)
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: bare, Ref: "main"},
		},
		Vars: map[string]string{"name": "world"},
	}

	Run(cfg, Options{DryRun: true})

	// Dry run should NOT create files in project dir
	if _, err := os.Stat("file.txt"); !os.IsNotExist(err) {
		t.Error("dry-run should not create files")
	}
}

func TestRun_Normal(t *testing.T) {
	bare := initBareRepo(t)
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: bare, Ref: "main"},
		},
		Vars: map[string]string{"name": "world"},
	}

	Run(cfg, Options{})

	// Files should be synced
	data, err := os.ReadFile("file.txt")
	if err != nil {
		t.Fatalf("file.txt should exist: %v", err)
	}
	if string(data) != "template content" {
		t.Errorf("file.txt: got %q", string(data))
	}

	// Var replacement should have happened
	data, err = os.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("hello.txt should exist: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("hello.txt: got %q, want %q", string(data), "hello world")
	}

	// Lock file should be created
	lock, err := config.LoadLock()
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	entry, ok := lock.Get("test")
	if !ok {
		t.Fatal("lock entry should exist")
	}
	if entry.Repo != bare {
		t.Errorf("lock repo: %q", entry.Repo)
	}
}

func TestRun_WithHooks(t *testing.T) {
	bare := initBareRepo(t)
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	beforeMarker := filepath.Join(dir, "before_ran")
	afterMarker := filepath.Join(dir, "after_ran")

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: bare, Ref: "main"},
		},
		Hooks: config.Hooks{
			Before: "touch " + beforeMarker,
			After:  "touch " + afterMarker,
		},
	}

	Run(cfg, Options{})

	if _, err := os.Stat(beforeMarker); err != nil {
		t.Error("before hook should have run")
	}
	if _, err := os.Stat(afterMarker); err != nil {
		t.Error("after hook should have run")
	}
}

func TestRun_InvalidRepo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "bad", Repo: "/nonexistent/repo.git", Ref: "main"},
		},
	}

	// Should not panic
	Run(cfg, Options{})
}

func TestDiff_Integration(t *testing.T) {
	bare := initBareRepo(t)
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	// Create a local file that differs from template
	os.WriteFile("file.txt", []byte("modified content"), 0644)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: bare, Ref: "main"},
		},
	}

	// Should not panic
	Diff(cfg)
}

func TestStatus_NoLock(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: "https://example.com/repo", Ref: "main"},
		},
	}

	// Should not panic
	Status(cfg)
}

func TestStatus_WithLock(t *testing.T) {
	bare := initBareRepo(t)
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	lock := config.LockFile{
		Entries: []config.LockEntry{
			{Name: "test", Repo: bare, Ref: "main", Commit: "abc123", Time: "2025-01-01T00:00:00Z"},
		},
	}
	config.SaveLock(lock)

	cfg := config.Config{
		Templates: []config.Template{
			{Name: "test", Repo: bare, Ref: "main"},
			{Name: "missing", Repo: bare, Ref: "main"},
		},
	}

	// Should not panic
	Status(cfg)
}

func TestInteractiveSyncDir_Yes(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(src, "update.txt"), []byte("updated"), 0644)
	os.WriteFile(filepath.Join(dst, "update.txt"), []byte("old"), 0644)

	// Redirect stdin with "y" answers
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("y\ny\n"))
		w.Close()
	}()

	interactiveSyncDir(src, dst, nil, nil)

	data, err := os.ReadFile(filepath.Join(dst, "new.txt"))
	if err != nil {
		t.Fatalf("new.txt should have been created: %v", err)
	}
	if string(data) != "new" {
		t.Errorf("new.txt: got %q", string(data))
	}

	data, _ = os.ReadFile(filepath.Join(dst, "update.txt"))
	if string(data) != "updated" {
		t.Errorf("update.txt: got %q", string(data))
	}
}

func TestInteractiveSyncDir_No(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "file.txt"), []byte("new"), 0644)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("n\n"))
		w.Close()
	}()

	interactiveSyncDir(src, dst, nil, nil)

	if _, err := os.Stat(filepath.Join(dst, "file.txt")); !os.IsNotExist(err) {
		t.Error("file should not be created when answering no")
	}
}

func TestInteractiveSyncDir_Quit(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(src, "b.txt"), []byte("b"), 0644)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("q\n"))
		w.Close()
	}()

	interactiveSyncDir(src, dst, nil, nil)

	// At most one file should be checked (the first one quit)
	aExists := false
	bExists := false
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err == nil {
		aExists = true
	}
	if _, err := os.Stat(filepath.Join(dst, "b.txt")); err == nil {
		bExists = true
	}
	if aExists && bExists {
		t.Error("quit should have stopped processing")
	}
}

func TestInteractiveSyncDir_SameFileSkipped(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Same content - should not prompt
	os.WriteFile(filepath.Join(src, "same.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(dst, "same.txt"), []byte("content"), 0644)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		// No input needed since file is unchanged
		w.Close()
	}()

	interactiveSyncDir(src, dst, nil, nil)
}

func TestInteractiveSyncDir_Excludes(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, "vendor"), 0755)
	os.WriteFile(filepath.Join(src, "vendor", "lib.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("y\n"))
		w.Close()
	}()

	interactiveSyncDir(src, dst, []string{"vendor"}, nil)

	if _, err := os.Stat(filepath.Join(dst, "vendor")); !os.IsNotExist(err) {
		t.Error("vendor should be excluded")
	}
}

func TestInteractiveSyncDir_WithVars(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "f.txt"), []byte("hello {ntpl:name}"), 0644)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("y\n"))
		w.Close()
	}()

	interactiveSyncDir(src, dst, nil, map[string]string{"name": "world"})

	data, _ := os.ReadFile(filepath.Join(dst, "f.txt"))
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("vars not replaced: %q", string(data))
	}
}
