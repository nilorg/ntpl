package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := Config{
		Templates: []Template{
			{Name: "base", Repo: "https://github.com/org/repo", Ref: "main"},
		},
		Sync: Sync{
			Include: []string{"."},
			Exclude: []string{"vendor"},
		},
		Vars: map[string]string{"project": "demo"},
		Hooks: Hooks{
			Before: "echo before",
			After:  "echo after",
		},
		Replace: map[string]ReplaceEntry{
			"org": {From: "oldorg", To: "neworg"},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Templates) != 1 || loaded.Templates[0].Name != "base" {
		t.Errorf("Templates mismatch: %+v", loaded.Templates)
	}
	if loaded.Templates[0].Repo != cfg.Templates[0].Repo {
		t.Errorf("Repo: got %q, want %q", loaded.Templates[0].Repo, cfg.Templates[0].Repo)
	}
	if loaded.Templates[0].Ref != "main" {
		t.Errorf("Ref: got %q, want %q", loaded.Templates[0].Ref, "main")
	}
	if len(loaded.Sync.Include) != 1 || loaded.Sync.Include[0] != "." {
		t.Errorf("Include: %+v", loaded.Sync.Include)
	}
	if len(loaded.Sync.Exclude) != 1 || loaded.Sync.Exclude[0] != "vendor" {
		t.Errorf("Exclude: %+v", loaded.Sync.Exclude)
	}
	if loaded.Vars["project"] != "demo" {
		t.Errorf("Vars: %+v", loaded.Vars)
	}
	if loaded.Hooks.Before != "echo before" {
		t.Errorf("Hooks.Before: %q", loaded.Hooks.Before)
	}
	if loaded.Replace["org"].From != "oldorg" || loaded.Replace["org"].To != "neworg" {
		t.Errorf("Replace: %+v", loaded.Replace)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing .ntpl.yaml")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	os.WriteFile(".ntpl.yaml", []byte(":::invalid yaml\n\t\t:::"), 0644)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := `templates:
  - name: t1
    repo: https://example.com/repo
    ref: v1.0
sync:
  exclude:
    - "*.log"
vars:
  key: value
`
	os.WriteFile(path, []byte(data), 0644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(cfg.Templates) != 1 || cfg.Templates[0].Name != "t1" {
		t.Errorf("Templates: %+v", cfg.Templates)
	}
	if cfg.Vars["key"] != "value" {
		t.Errorf("Vars: %+v", cfg.Vars)
	}
}

func TestLoadFrom_MissingFile(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMergeSync_LocalTakesPrecedence(t *testing.T) {
	local := Config{
		Sync: Sync{
			Include: []string{"src/"},
			Exclude: []string{"dist/"},
		},
		Vars: map[string]string{"name": "local-val", "only_local": "yes"},
	}
	remote := Config{
		Sync: Sync{
			Include: []string{"lib/"},
			Exclude: []string{"build/"},
		},
		Vars: map[string]string{"name": "remote-val", "only_remote": "yes"},
	}

	s, vars := MergeSync(local, remote)

	// Local include is non-empty, so it should win
	if len(s.Include) != 1 || s.Include[0] != "src/" {
		t.Errorf("Include should be local: %+v", s.Include)
	}
	if len(s.Exclude) != 1 || s.Exclude[0] != "dist/" {
		t.Errorf("Exclude should be local: %+v", s.Exclude)
	}
	// Local vars override remote
	if vars["name"] != "local-val" {
		t.Errorf("name should be local-val, got %q", vars["name"])
	}
	if vars["only_local"] != "yes" {
		t.Errorf("only_local missing")
	}
	if vars["only_remote"] != "yes" {
		t.Errorf("only_remote missing")
	}
}

func TestMergeSync_EmptyLocalUsesRemote(t *testing.T) {
	local := Config{}
	remote := Config{
		Sync: Sync{
			Include: []string{"lib/"},
			Exclude: []string{"build/"},
		},
		Vars: map[string]string{"remote_key": "remote_val"},
	}

	s, vars := MergeSync(local, remote)

	if len(s.Include) != 1 || s.Include[0] != "lib/" {
		t.Errorf("Include should fall back to remote: %+v", s.Include)
	}
	if len(s.Exclude) != 1 || s.Exclude[0] != "build/" {
		t.Errorf("Exclude should fall back to remote: %+v", s.Exclude)
	}
	if vars["remote_key"] != "remote_val" {
		t.Errorf("remote vars not merged: %+v", vars)
	}
}

func TestMergeSync_BothEmpty(t *testing.T) {
	s, vars := MergeSync(Config{}, Config{})
	if len(s.Include) != 0 || len(s.Exclude) != 0 {
		t.Errorf("expected empty sync: %+v", s)
	}
	if len(vars) != 0 {
		t.Errorf("expected empty vars: %+v", vars)
	}
}

func TestReplaceEntry_UnmarshalYAML_StringShorthand(t *testing.T) {
	input := `"newvalue"`
	var r ReplaceEntry
	if err := yaml.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.From != "" {
		t.Errorf("From should be empty, got %q", r.From)
	}
	if r.To != "newvalue" {
		t.Errorf("To should be newvalue, got %q", r.To)
	}
}

func TestReplaceEntry_UnmarshalYAML_FullStruct(t *testing.T) {
	input := `from: oldval
to: newval`
	var r ReplaceEntry
	if err := yaml.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.From != "oldval" {
		t.Errorf("From: got %q, want %q", r.From, "oldval")
	}
	if r.To != "newval" {
		t.Errorf("To: got %q, want %q", r.To, "newval")
	}
}

func TestReplaceEntry_UnmarshalYAML_InMap(t *testing.T) {
	input := `
org: myorg
module:
  from: old/module
  to: new/module
`
	var m map[string]ReplaceEntry
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("unmarshal map: %v", err)
	}

	org, ok := m["org"]
	if !ok {
		t.Fatal("missing org key")
	}
	if org.To != "myorg" || org.From != "" {
		t.Errorf("org: %+v", org)
	}

	mod, ok := m["module"]
	if !ok {
		t.Fatal("missing module key")
	}
	if mod.From != "old/module" || mod.To != "new/module" {
		t.Errorf("module: %+v", mod)
	}
}

func TestLockFile_SetAndGet(t *testing.T) {
	var lf LockFile

	// Add
	lf.Set(LockEntry{Name: "t1", Repo: "r1", Ref: "main", Commit: "abc"})
	entry, ok := lf.Get("t1")
	if !ok {
		t.Fatal("expected to find t1")
	}
	if entry.Commit != "abc" {
		t.Errorf("Commit: got %q, want %q", entry.Commit, "abc")
	}

	// Update existing
	lf.Set(LockEntry{Name: "t1", Repo: "r1", Ref: "main", Commit: "def"})
	entry, ok = lf.Get("t1")
	if !ok {
		t.Fatal("expected to find t1 after update")
	}
	if entry.Commit != "def" {
		t.Errorf("Commit after update: got %q, want %q", entry.Commit, "def")
	}
	if len(lf.Entries) != 1 {
		t.Errorf("should not duplicate: len=%d", len(lf.Entries))
	}

	// Multiple entries
	lf.Set(LockEntry{Name: "t2", Repo: "r2", Commit: "ghi"})
	if len(lf.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(lf.Entries))
	}

	// Not found
	_, ok = lf.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent entry")
	}
}

func TestLoadLockAndSaveLock(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	lock := LockFile{
		Entries: []LockEntry{
			{Name: "a", Repo: "r", Ref: "v1", Commit: "111", Time: "2025-01-01T00:00:00Z"},
		},
	}

	if err := SaveLock(lock); err != nil {
		t.Fatalf("SaveLock: %v", err)
	}

	loaded, err := LoadLock()
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Commit != "111" {
		t.Errorf("Commit: %q", loaded.Entries[0].Commit)
	}
}

func TestLoadLock_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	lock, err := LoadLock()
	if err != nil {
		t.Fatalf("LoadLock should not error on missing file: %v", err)
	}
	if len(lock.Entries) != 0 {
		t.Errorf("expected empty entries")
	}
}

func TestLoadIgnore(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	content := `# comment
*.log
  
vendor/
# another comment
tmp
`
	os.WriteFile(".ntplignore", []byte(content), 0644)

	patterns := LoadIgnore()
	expected := []string{"*.log", "vendor/", "tmp"}
	if len(patterns) != len(expected) {
		t.Fatalf("expected %d patterns, got %d: %v", len(expected), len(patterns), patterns)
	}
	for i, p := range expected {
		if patterns[i] != p {
			t.Errorf("pattern[%d]: got %q, want %q", i, patterns[i], p)
		}
	}
}

func TestLoadIgnore_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	patterns := LoadIgnore()
	if patterns != nil {
		t.Errorf("expected nil for missing file, got %v", patterns)
	}
}

func TestSave_RoundTrip_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	cfg := Config{}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Templates) != 0 {
		t.Errorf("expected empty templates: %+v", loaded)
	}
}
