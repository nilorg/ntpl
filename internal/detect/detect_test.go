package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRules_BuiltinRulesExist(t *testing.T) {
	rules := LoadRules()
	if len(rules) == 0 {
		t.Fatal("expected builtin rules to be loaded")
	}

	names := make(map[string]bool)
	for _, r := range rules {
		names[r.Name] = true
	}

	expected := []string{"go", "node", "python", "java", "rust", "docker", "git"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing builtin rule: %s", name)
		}
	}
}

func TestLoadRules_PriorityOrder(t *testing.T) {
	rules := LoadRules()
	for i := 1; i < len(rules); i++ {
		pi := rulePriority(rules[i-1])
		pj := rulePriority(rules[i])
		if pi > pj {
			t.Errorf("rules not sorted by priority: %s(%d) > %s(%d)",
				rules[i-1].Name, pi, rules[i].Name, pj)
		}
		// Same priority: sorted by name
		if pi == pj && rules[i-1].Name > rules[i].Name {
			t.Errorf("same-priority rules not sorted by name: %s > %s",
				rules[i-1].Name, rules[i].Name)
		}
	}
}

func TestRulePriority_Default(t *testing.T) {
	r := Rule{Name: "test"}
	if p := rulePriority(r); p != 50 {
		t.Errorf("default priority: got %d, want 50", p)
	}
}

func TestRulePriority_Explicit(t *testing.T) {
	r := Rule{Name: "test", Priority: 10}
	if p := rulePriority(r); p != 10 {
		t.Errorf("explicit priority: got %d, want 10", p)
	}
}

func TestDetect_GoProject(t *testing.T) {
	dir := t.TempDir()
	gomod := `module github.com/testorg/testproject

go 1.21
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	found := make(map[string]string)
	for _, v := range vars {
		found[v.Key] = v.Value
	}

	if found["org"] != "testorg" {
		t.Errorf("org: got %q, want %q", found["org"], "testorg")
	}
	if found["project_name"] != "testproject" {
		t.Errorf("project_name: got %q, want %q", found["project_name"], "testproject")
	}
	if found["module"] != "github.com/testorg/testproject" {
		t.Errorf("module: got %q, want %q", found["module"], "github.com/testorg/testproject")
	}
	if found["go_version"] != "1.21" {
		t.Errorf("go_version: got %q, want %q", found["go_version"], "1.21")
	}
}

func TestDetect_NodeProject(t *testing.T) {
	dir := t.TempDir()
	pkg := `{
  "name": "my-app",
  "version": "2.0.0"
}
`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	found := make(map[string]string)
	for _, v := range vars {
		found[v.Key] = v.Value
	}

	if found["project_name"] != "my-app" {
		t.Errorf("project_name: got %q, want %q", found["project_name"], "my-app")
	}
	if found["version"] != "2.0.0" {
		t.Errorf("version: got %q, want %q", found["version"], "2.0.0")
	}
}

func TestDetect_PythonProject(t *testing.T) {
	dir := t.TempDir()
	setup := `from setuptools import setup
setup(
    name="my-project",
    version="1.2.3",
)
`
	os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setup), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	found := make(map[string]string)
	for _, v := range vars {
		found[v.Key] = v.Value
	}

	if found["project_name"] != "my-project" {
		t.Errorf("project_name: got %q, want %q", found["project_name"], "my-project")
	}
	if found["version"] != "1.2.3" {
		t.Errorf("version: got %q, want %q", found["version"], "1.2.3")
	}
}

func TestDetect_DockerProject(t *testing.T) {
	dir := t.TempDir()
	dockerfile := `FROM golang:1.21-alpine
EXPOSE 8080
`
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	found := make(map[string]string)
	for _, v := range vars {
		found[v.Key] = v.Value
	}

	if found["port"] != "8080" {
		t.Errorf("port: got %q, want %q", found["port"], "8080")
	}
}

func TestDetect_FirstMatchPerKeyWins(t *testing.T) {
	dir := t.TempDir()
	// Write go.mod with org
	gomod := `module github.com/firstorg/proj

go 1.21
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	// Write .git/config with different org — git rule has lower priority (90)
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	gitConfig := `[remote "origin"]
	url = https://github.com/secondorg/proj.git
`
	os.WriteFile(filepath.Join(gitDir, "config"), []byte(gitConfig), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	found := make(map[string]string)
	for _, v := range vars {
		found[v.Key] = v.Value
	}

	// go rule (priority 10) should win over git rule (priority 90)
	if found["org"] != "firstorg" {
		t.Errorf("org should be from go rule (firstorg), got %q", found["org"])
	}
}

func TestDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	rules := LoadRules()
	vars := Detect(rules, dir)
	if len(vars) != 0 {
		t.Errorf("expected no vars for empty dir, got %v", vars)
	}
}

func TestDetect_VarProperties(t *testing.T) {
	dir := t.TempDir()
	gomod := `module github.com/myorg/myproj

go 1.22
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	rules := LoadRules()
	vars := Detect(rules, dir)

	for _, v := range vars {
		if v.Key == "" {
			t.Error("Var.Key should not be empty")
		}
		if v.Value == "" {
			t.Error("Var.Value should not be empty")
		}
		if v.Source == "" {
			t.Error("Var.Source should not be empty")
		}
		if v.Rule == "" {
			t.Error("Var.Rule should not be empty")
		}
	}
}

func TestDetect_InvalidRegexSkipped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	rules := []Rule{
		{
			Name: "bad",
			Files: []FileRule{
				{
					Path:     "test.txt",
					Patterns: []Pattern{{Regexp: "[invalid"}},
				},
			},
		},
	}

	// Should not panic
	vars := Detect(rules, dir)
	if len(vars) != 0 {
		t.Errorf("expected no vars for invalid regex, got %v", vars)
	}
}

func TestLoadDirRules_NonexistentDir(t *testing.T) {
	ruleMap := make(map[string]Rule)
	loadDirRules("/nonexistent/path/rules", ruleMap)
	if len(ruleMap) != 0 {
		t.Errorf("expected empty ruleMap: %+v", ruleMap)
	}
}

func TestLoadDirRules_Override(t *testing.T) {
	dir := t.TempDir()
	ruleYAML := `name: go
description: Custom go rule
priority: 5
files: []
`
	os.WriteFile(filepath.Join(dir, "go.yaml"), []byte(ruleYAML), 0644)

	ruleMap := make(map[string]Rule)
	ruleMap["go"] = Rule{Name: "go", Description: "Builtin", Priority: 10}

	loadDirRules(dir, ruleMap)

	if ruleMap["go"].Description != "Custom go rule" {
		t.Errorf("rule should be overridden: %+v", ruleMap["go"])
	}
	if ruleMap["go"].Priority != 5 {
		t.Errorf("priority should be 5, got %d", ruleMap["go"].Priority)
	}
}

func TestLoadDirRules_SkipsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::invalid"), 0644)

	ruleMap := make(map[string]Rule)
	loadDirRules(dir, ruleMap)
	if len(ruleMap) != 0 {
		t.Errorf("should skip invalid yaml: %+v", ruleMap)
	}
}

func TestLoadDirRules_SkipsEmptyName(t *testing.T) {
	dir := t.TempDir()
	ruleYAML := `description: no name
priority: 1
files: []
`
	os.WriteFile(filepath.Join(dir, "noname.yaml"), []byte(ruleYAML), 0644)

	ruleMap := make(map[string]Rule)
	loadDirRules(dir, ruleMap)
	if len(ruleMap) != 0 {
		t.Errorf("should skip rule without name: %+v", ruleMap)
	}
}

func TestLoadDirRules_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "rule.json"), []byte(`{"name":"x"}`), 0644)

	ruleMap := make(map[string]Rule)
	loadDirRules(dir, ruleMap)
	if len(ruleMap) != 0 {
		t.Errorf("should skip non-yaml files: %+v", ruleMap)
	}
}
