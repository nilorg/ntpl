package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nilorg/ntpl/internal/config"
	"github.com/nilorg/ntpl/internal/detect"

	"github.com/spf13/cobra"
)

var (
	replaceDryRun  bool
	replaceSuggest bool
)

var replaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "Replace source values with your own in project files",
	Run: func(cmd *cobra.Command, args []string) {
		rules := detect.LoadRules()
		detected := detect.Detect(rules, ".")

		var replacements []replaceItem
		depDirs := config.DefaultReplaceExcludes

		if replaceSuggest {
			if len(detected) == 0 {
				fmt.Println("no variables detected")
				return
			}

			fmt.Println("detected variables:")
			reader := bufio.NewReader(os.Stdin)
			for _, v := range detected {
				fmt.Printf("  %s = %s (from %s)\n", v.Key, v.Value, v.Source)
				fmt.Printf("    replace with (enter to skip): ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(answer)
				if answer != "" && answer != v.Value {
					replacements = append(replacements, replaceItem{
						key:  v.Key,
						from: v.Value,
						to:   answer,
					})
				}
			}

			if len(replacements) > 0 {
				fmt.Print("\nsave to .ntpl.yaml? [y/n] ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "y" || answer == "yes" {
					saveReplaceConfig(replacements)
				}
			}
		} else {
			cfg, err := config.Load()
			if err != nil {
				fmt.Println("error:", err)
				return
			}
			depDirs = cfg.Replace.GetExcludes()

			if len(cfg.Replace.Rules) == 0 {
				fmt.Println("no replace rules in .ntpl.yaml, use --suggest for interactive mode")
				return
			}

			detectedMap := make(map[string]string)
			for _, v := range detected {
				detectedMap[v.Key] = v.Value
			}

			for key, entry := range cfg.Replace.Rules {
				from := entry.From
				if from == "" {
					from = detectedMap[key]
				}
				if from == "" {
					fmt.Printf("warning: could not detect source value for %q, skipping\n", key)
					continue
				}
				if from == entry.To {
					continue
				}
				replacements = append(replacements, replaceItem{
					key:  key,
					from: from,
					to:   entry.To,
				})
			}
		}

		if len(replacements) == 0 {
			fmt.Println("nothing to replace")
			return
		}

		// Sort by from length descending to avoid partial replacements.
		sort.Slice(replacements, func(i, j int) bool {
			return len(replacements[i].from) > len(replacements[j].from)
		})

		fmt.Println("\nreplacements:")
		for _, r := range replacements {
			fmt.Printf("  %s: %q → %q\n", r.key, r.from, r.to)
		}

		for _, r := range replacements {
			if len(r.from) <= 3 {
				fmt.Printf("warning: %q source value %q is very short, may cause false replacements\n", r.key, r.from)
			}
		}

		totalFiles := 0
		totalReplacements := 0

		filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if path == "." {
				return nil
			}

			if config.IsExcluded(path, config.BuiltinExcludes) || config.IsExcluded(path, depDirs) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Skip binary files.
			check := data
			if len(check) > 8192 {
				check = check[:8192]
			}
			if bytes.Contains(check, []byte{0}) {
				return nil
			}

			newData := data
			fileReplacements := 0
			for _, r := range replacements {
				n := bytes.Count(newData, []byte(r.from))
				if n > 0 {
					newData = bytes.ReplaceAll(newData, []byte(r.from), []byte(r.to))
					fileReplacements += n
				}
			}

			if fileReplacements == 0 {
				return nil
			}

			totalFiles++
			totalReplacements += fileReplacements

			if replaceDryRun {
				fmt.Printf("  %s (%d replacements)\n", path, fileReplacements)
				return nil
			}

			info, _ := d.Info()
			if err := os.WriteFile(path, newData, info.Mode()); err != nil {
				fmt.Printf("  error writing %s: %s\n", path, err)
			} else {
				fmt.Printf("  %s (%d replacements)\n", path, fileReplacements)
			}

			return nil
		})

		fmt.Println()
		if replaceDryRun {
			fmt.Printf("dry-run: would replace %d occurrences in %d files\n", totalReplacements, totalFiles)
		} else {
			fmt.Printf("replaced %d occurrences in %d files\n", totalReplacements, totalFiles)
		}

		// Phase 2: rename directories and files whose names contain from values.
		var renames []rename
		filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == "." {
				return err
			}
			if config.IsExcluded(path, config.BuiltinExcludes) || config.IsExcluded(path, depDirs) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			newName := name
			for _, r := range replacements {
				newName = strings.ReplaceAll(newName, r.from, r.to)
			}
			if newName != name {
				dir := filepath.Dir(path)
				renames = append(renames, rename{
					oldPath: path,
					newPath: filepath.Join(dir, newName),
					isDir:   d.IsDir(),
				})
			}
			return nil
		})

		if len(renames) > 0 {
			// Sort by path depth descending so children are renamed before parents.
			sort.Slice(renames, func(i, j int) bool {
				di := strings.Count(renames[i].oldPath, string(filepath.Separator))
				dj := strings.Count(renames[j].oldPath, string(filepath.Separator))
				if di != dj {
					return di > dj
				}
				return renames[i].oldPath > renames[j].oldPath
			})

			fmt.Println("\nrenames:")
			for _, r := range renames {
				if replaceDryRun {
					fmt.Printf("  %s → %s\n", r.oldPath, r.newPath)
				} else {
					if err := os.Rename(r.oldPath, r.newPath); err != nil {
						fmt.Printf("  error renaming %s: %s\n", r.oldPath, err)
					} else {
						fmt.Printf("  %s → %s\n", r.oldPath, r.newPath)
					}
				}
			}

			if replaceDryRun {
				fmt.Printf("dry-run: would rename %d paths\n", len(renames))
			} else {
				fmt.Printf("renamed %d paths\n", len(renames))
			}
		}
	},
}

type replaceItem struct {
	key  string
	from string
	to   string
}

type rename struct {
	oldPath string
	newPath string
	isDir   bool
}

func saveReplaceConfig(repls []replaceItem) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("warning: could not load config:", err)
		return
	}

	if cfg.Replace.Rules == nil {
		cfg.Replace.Rules = make(map[string]config.ReplaceEntry)
	}
	for _, r := range repls {
		cfg.Replace.Rules[r.key] = config.ReplaceEntry{
			From: r.from,
			To:   r.to,
		}
	}

	if err := config.Save(cfg); err != nil {
		fmt.Println("warning: could not save config:", err)
	} else {
		fmt.Println("saved replace config to .ntpl.yaml")
	}
}

func init() {
	replaceCmd.Flags().BoolVar(&replaceDryRun, "dry-run", false, "show what would be replaced without making changes")
	replaceCmd.Flags().BoolVar(&replaceSuggest, "suggest", false, "interactively detect and replace variables")
	rootCmd.AddCommand(replaceCmd)
}
