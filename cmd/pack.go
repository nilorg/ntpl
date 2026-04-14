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
	"gopkg.in/yaml.v3"
)

var (
	packOutput  string
	packVars    []string
	packSuggest bool
	packDryRun  bool
)

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Pack current project as a template with {ntpl:key} placeholders",
	Run: func(cmd *cobra.Command, args []string) {
		if packOutput == "" {
			fmt.Println("error: output directory is required, use -o <dir>")
			return
		}

		vars := parseVarFlags(packVars)

		if packSuggest {
			rules := detect.LoadRules()
			detected := detect.Detect(rules, ".")

			if len(detected) == 0 && len(vars) == 0 {
				fmt.Println("no variables detected and none specified")
				return
			}

			if len(detected) > 0 {
				fmt.Println("detected variables:")
				for _, v := range detected {
					if _, exists := vars[v.Key]; exists {
						fmt.Printf("  %-20s = %s (from %s, overridden by --var)\n", v.Key, v.Value, v.Source)
					} else {
						fmt.Printf("  %-20s = %s (from %s)\n", v.Key, v.Value, v.Source)
					}
				}

				for _, v := range detected {
					if _, exists := vars[v.Key]; !exists {
						vars[v.Key] = v.Value
					}
				}

				fmt.Printf("\nfinal variables (%d):\n", len(vars))
				keys := make([]string, 0, len(vars))
				for k := range vars {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("  %-20s = %s\n", k, vars[k])
				}

				fmt.Print("\naccept? [y/n] ")
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("aborted")
					return
				}
			}
		}

		if len(vars) == 0 {
			fmt.Println("error: no variables specified, use --var key=value or --suggest")
			return
		}

		// Sort by value length descending to avoid partial replacements.
		type kv struct {
			key   string
			value string
		}
		pairs := make([]kv, 0, len(vars))
		for k, v := range vars {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return len(pairs[i].value) > len(pairs[j].value)
		})

		for _, p := range pairs {
			if len(p.value) <= 3 {
				fmt.Printf("warning: var %q value %q is very short, may cause false replacements\n", p.key, p.value)
			}
		}

		fileCount := 0
		totalReplacements := 0

		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if path == "." {
				return nil
			}

			if config.IsExcluded(path, config.BuiltinExcludes) || config.IsExcluded(path, config.DefaultReplaceExcludes) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			target := filepath.Join(packOutput, path)

			if d.IsDir() {
				if !packDryRun {
					return os.MkdirAll(target, 0755)
				}
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Skip binary files (null bytes in first 8K).
			check := data
			if len(check) > 8192 {
				check = check[:8192]
			}
			isBinary := bytes.Contains(check, []byte{0})

			fileReplacements := 0
			if !isBinary {
				for _, p := range pairs {
					n := bytes.Count(data, []byte(p.value))
					if n > 0 {
						data = bytes.ReplaceAll(data, []byte(p.value), []byte("{ntpl:"+p.key+"}"))
						fileReplacements += n
					}
				}
			}

			totalReplacements += fileReplacements
			fileCount++

			if packDryRun {
				if fileReplacements > 0 {
					fmt.Printf("  %s (%d replacements)\n", path, fileReplacements)
				} else {
					fmt.Printf("  %s\n", path)
				}
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			return os.WriteFile(target, data, info.Mode())
		})

		if err != nil {
			fmt.Println("error:", err)
			return
		}

		// Generate .ntpl.yaml in output directory.
		if !packDryRun {
			outCfg := config.Config{
				Sync: config.Sync{
					Vars: vars,
				},
			}
			yamlData, err := yaml.Marshal(outCfg)
			if err == nil {
				os.WriteFile(filepath.Join(packOutput, ".ntpl.yaml"), yamlData, 0644)
			}
		}

		if packDryRun {
			fmt.Printf("\ndry-run: would pack %d files to %s\n", fileCount, packOutput)
		} else {
			fmt.Printf("\npacked %d files to %s\n", fileCount, packOutput)
		}
		if totalReplacements > 0 {
			fmt.Printf("replaced %d occurrences\n", totalReplacements)
		}
	},
}

func parseVarFlags(flags []string) map[string]string {
	vars := make(map[string]string)
	for _, f := range flags {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
	}
	return vars
}

func init() {
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "output directory for the template")
	packCmd.Flags().StringArrayVar(&packVars, "var", nil, "variable to replace (key=value, repeatable)")
	packCmd.Flags().BoolVar(&packSuggest, "suggest", false, "auto-detect variables from project files")
	packCmd.Flags().BoolVar(&packDryRun, "dry-run", false, "show what would be packed without making changes")
	rootCmd.AddCommand(packCmd)
}
