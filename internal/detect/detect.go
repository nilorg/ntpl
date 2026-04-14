package detect

import (
	"embed"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed rules/*.yaml
var builtinRulesFS embed.FS

// Rule defines a detection rule for extracting variables from project files.
type Rule struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Priority    int        `yaml:"priority"`
	Files       []FileRule `yaml:"files"`
}

// FileRule defines which file to scan and what patterns to match.
type FileRule struct {
	Path     string    `yaml:"path"`
	Patterns []Pattern `yaml:"patterns"`
}

// Pattern defines a regexp with named capture groups to extract variables.
type Pattern struct {
	Regexp      string `yaml:"regexp"`
	Description string `yaml:"description"`
}

// Var represents a detected variable.
type Var struct {
	Key    string
	Value  string
	Source string // relative file path
	Rule   string // rule name
}

// LoadRules loads detection rules from builtin, user, and project directories.
// Same-name rules: project > user > builtin.
func LoadRules() []Rule {
	ruleMap := make(map[string]Rule)

	// 1. Builtin (lowest priority)
	if entries, err := builtinRulesFS.ReadDir("rules"); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			data, err := builtinRulesFS.ReadFile("rules/" + e.Name())
			if err != nil {
				continue
			}
			var r Rule
			if err := yaml.Unmarshal(data, &r); err != nil || r.Name == "" {
				continue
			}
			ruleMap[r.Name] = r
		}
	}

	// 2. User rules (~/.config/ntpl/rules/)
	if home, err := os.UserHomeDir(); err == nil {
		loadDirRules(filepath.Join(home, ".config", "ntpl", "rules"), ruleMap)
	}

	// 3. Project rules (.ntpl/rules/) — highest priority
	loadDirRules(filepath.Join(".ntpl", "rules"), ruleMap)

	rules := make([]Rule, 0, len(ruleMap))
	for _, r := range ruleMap {
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool {
		pi, pj := rulePriority(rules[i]), rulePriority(rules[j])
		if pi != pj {
			return pi < pj
		}
		return rules[i].Name < rules[j].Name
	})

	return rules
}

func rulePriority(r Rule) int {
	if r.Priority == 0 {
		return 50
	}
	return r.Priority
}

func loadDirRules(dir string, ruleMap map[string]Rule) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r Rule
		if err := yaml.Unmarshal(data, &r); err != nil || r.Name == "" {
			continue
		}
		ruleMap[r.Name] = r
	}
}

// Detect scans the directory for variables using the provided rules.
// First match per key wins.
func Detect(rules []Rule, dir string) []Var {
	seen := make(map[string]bool)
	var vars []Var

	for _, rule := range rules {
		for _, file := range rule.Files {
			matches, err := filepath.Glob(filepath.Join(dir, file.Path))
			if err != nil || len(matches) == 0 {
				continue
			}

			for _, match := range matches {
				data, err := os.ReadFile(match)
				if err != nil {
					continue
				}

				rel, _ := filepath.Rel(dir, match)
				lines := strings.Split(string(data), "\n")

				for _, pattern := range file.Patterns {
					re, err := regexp.Compile(pattern.Regexp)
					if err != nil {
						continue
					}

					for _, line := range lines {
						m := re.FindStringSubmatch(line)
						if m == nil {
							continue
						}

						for i, name := range re.SubexpNames() {
							if i == 0 || name == "" || m[i] == "" {
								continue
							}
							if seen[name] {
								continue
							}
							seen[name] = true
							vars = append(vars, Var{
								Key:    name,
								Value:  m[i],
								Source: rel,
								Rule:   rule.Name,
							})
						}
					}
				}
			}
		}
	}

	return vars
}
