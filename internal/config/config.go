package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Templates []Template `yaml:"templates"`
	Sync      Sync       `yaml:"sync"`
}

type Template struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
}

type Sync struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
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
