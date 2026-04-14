package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// BuiltinExcludes contains paths that ntpl should always skip (own files + VCS).
var BuiltinExcludes = []string{
	".ntpl", ".ntpl.yaml", ".ntpl.lock", ".ntplignore",
	".git",
}

// DefaultReplaceExcludes contains default directories excluded from replace.
var DefaultReplaceExcludes = []string{
	"vendor", "node_modules",
}

// IsExcluded checks if a relative path matches any of the given exclude patterns.
func IsExcluded(rel string, excludes []string) bool {
	base := filepath.Base(rel)
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, rel); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if strings.HasPrefix(rel, pattern+string(filepath.Separator)) || rel == pattern {
			return true
		}
	}
	return false
}

type Config struct {
	Templates []Template    `yaml:"templates"`
	Sync      Sync          `yaml:"sync"`
	Replace   ReplaceConfig `yaml:"replace"`
}

// ReplaceConfig holds settings for the replace command.
type ReplaceConfig struct {
	Exclude []string                `yaml:"exclude"`
	Rules   map[string]ReplaceEntry `yaml:"rules"`
}

// GetExcludes returns configured replace excludes, or defaults if not set.
func (r ReplaceConfig) GetExcludes() []string {
	if len(r.Exclude) > 0 {
		return r.Exclude
	}
	return DefaultReplaceExcludes
}

type Hooks struct {
	Before string `yaml:"before"`
	After  string `yaml:"after"`
}

type ReplaceEntry struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (r *ReplaceEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		r.To = value.Value
		return nil
	}
	type plain ReplaceEntry
	return value.Decode((*plain)(r))
}

type Template struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
}

type Sync struct {
	Include []string          `yaml:"include"`
	Exclude []string          `yaml:"exclude"`
	Vars    map[string]string `yaml:"vars"`
	Hooks   Hooks             `yaml:"hooks"`
}

type LockEntry struct {
	Name   string `yaml:"name"`
	Repo   string `yaml:"repo"`
	Ref    string `yaml:"ref"`
	Commit string `yaml:"commit"`
	Time   string `yaml:"time"`
}

type LockFile struct {
	Entries []LockEntry `yaml:"entries"`
}

func Load() (Config, error) {
	data, err := os.ReadFile(".ntpl.yaml")
	if err != nil {
		return Config{}, fmt.Errorf("read .ntpl.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse .ntpl.yaml: %w", err)
	}
	return cfg, nil
}

// LoadFrom loads a Config from the specified file path.
func LoadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// MergeSync merges local and remote configs. Local settings take precedence.
// Remote hooks are never merged (security).
func MergeSync(local, remote Config) Sync {
	s := local.Sync
	if len(s.Include) == 0 && len(remote.Sync.Include) > 0 {
		s.Include = remote.Sync.Include
	}
	if len(s.Exclude) == 0 && len(remote.Sync.Exclude) > 0 {
		s.Exclude = remote.Sync.Exclude
	}
	vars := make(map[string]string)
	for k, v := range remote.Sync.Vars {
		vars[k] = v
	}
	for k, v := range local.Sync.Vars {
		vars[k] = v
	}
	s.Vars = vars
	// Always use local hooks, never merge remote hooks.
	s.Hooks = local.Sync.Hooks
	return s
}

func Save(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(".ntpl.yaml", data, 0644); err != nil {
		return fmt.Errorf("write .ntpl.yaml: %w", err)
	}
	return nil
}

func LoadLock() (LockFile, error) {
	data, err := os.ReadFile(".ntpl.lock")
	if err != nil {
		if os.IsNotExist(err) {
			return LockFile{}, nil
		}
		return LockFile{}, fmt.Errorf("read .ntpl.lock: %w", err)
	}

	var lock LockFile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return LockFile{}, fmt.Errorf("parse .ntpl.lock: %w", err)
	}
	return lock, nil
}

func SaveLock(lock LockFile) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	if err := os.WriteFile(".ntpl.lock", data, 0644); err != nil {
		return fmt.Errorf("write .ntpl.lock: %w", err)
	}
	return nil
}

func (lf *LockFile) Set(entry LockEntry) {
	for i, e := range lf.Entries {
		if e.Name == entry.Name {
			lf.Entries[i] = entry
			return
		}
	}
	lf.Entries = append(lf.Entries, entry)
}

func (lf *LockFile) Get(name string) (LockEntry, bool) {
	for _, e := range lf.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return LockEntry{}, false
}

// LoadIgnore reads .ntplignore file and returns patterns (one per line).
func LoadIgnore() []string {
	f, err := os.Open(".ntplignore")
	if err != nil {
		return nil
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}
