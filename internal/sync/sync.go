package sync

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nilorg/ntpl/internal/config"
	"github.com/nilorg/ntpl/internal/git"
)

const baseDir = ".ntpl"

var varRe = regexp.MustCompile(`\{ntpl:(\w+)\}`)

// Options controls sync behavior.
type Options struct {
	DryRun      bool
	Interactive bool
}

func mergeExcludes(cfg config.Sync) []string {
	excludes := append(config.BuiltinExcludes, cfg.Exclude...)
	excludes = append(excludes, config.LoadIgnore()...)
	return excludes
}

func templateDir(tpl config.Template) string {
	return filepath.Join(baseDir, "template", tpl.Name)
}

func replaceVars(data []byte, vars map[string]string) []byte {
	if len(vars) == 0 {
		return data
	}
	return varRe.ReplaceAllFunc(data, func(match []byte) []byte {
		key := string(varRe.FindSubmatch(match)[1])
		if val, ok := vars[key]; ok {
			return []byte(val)
		}
		return match
	})
}

func runHook(name, script string) error {
	if script == "" {
		return nil
	}
	fmt.Printf("running %s hook: %s\n", name, script)
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s hook failed: %w", name, err)
	}
	return nil
}

func loadRemoteDefaults(cfg config.Config, dir string) config.Sync {
	remotePath := filepath.Join(dir, ".ntpl.yaml")
	remote, err := config.LoadFrom(remotePath)
	if err != nil {
		return cfg.Sync
	}
	fmt.Println("  loaded remote config defaults from template")
	return config.MergeSync(cfg, remote)
}

func Run(cfg config.Config, opts Options) {
	fmt.Println("sync template...")

	if !opts.DryRun {
		if err := runHook("before", cfg.Sync.Hooks.Before); err != nil {
			fmt.Println("error:", err)
			return
		}
	}

	lock, err := config.LoadLock()
	if err != nil {
		fmt.Println("warning: load lock file:", err)
	}

	for _, tpl := range cfg.Templates {
		dir := templateDir(tpl)
		fmt.Printf("\n[%s] fetching %s (ref: %s)\n", tpl.Name, tpl.Repo, tpl.Ref)

		commit, err := git.Export(tpl.Repo, tpl.Ref, dir)
		if err != nil {
			fmt.Printf("[%s] git export failed: %s\n", tpl.Name, err)
			continue
		}

		syncCfg := loadRemoteDefaults(cfg, dir)
		excludes := mergeExcludes(syncCfg)

		paths := syncCfg.Include
		if len(paths) == 0 {
			paths = []string{"."}
		}

		for _, path := range paths {
			src := filepath.Join(dir, path)
			dst := filepath.Join(".", path)

			if opts.DryRun {
				fmt.Printf("[%s] dry-run: %s -> %s\n", tpl.Name, src, dst)
				dryRunDir(src, dst, excludes, syncCfg.Vars)
			} else if opts.Interactive {
				fmt.Printf("[%s] interactive sync: %s -> %s\n", tpl.Name, src, dst)
				interactiveSyncDir(src, dst, excludes, syncCfg.Vars)
			} else {
				fmt.Printf("[%s] sync: %s -> %s\n", tpl.Name, src, dst)
				if err := syncDir(src, dst, excludes, syncCfg.Vars); err != nil {
					fmt.Printf("[%s] sync failed for %s: %s\n", tpl.Name, path, err)
				}
			}
		}

		if !opts.DryRun {
			lock.Set(config.LockEntry{
				Name:   tpl.Name,
				Repo:   tpl.Repo,
				Ref:    tpl.Ref,
				Commit: commit,
				Time:   time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	if !opts.DryRun {
		if err := config.SaveLock(lock); err != nil {
			fmt.Println("warning: save lock file:", err)
		}
		fmt.Println("\nsync done")

		if err := runHook("after", cfg.Sync.Hooks.After); err != nil {
			fmt.Println("error:", err)
		}
	} else {
		fmt.Println("\ndry-run complete, no files changed")
	}
}

func Diff(cfg config.Config) {
	fmt.Println("diff template...")

	for _, tpl := range cfg.Templates {
		dir := templateDir(tpl)
		fmt.Printf("\n[%s] fetching %s (ref: %s)\n", tpl.Name, tpl.Repo, tpl.Ref)

		if _, err := git.Export(tpl.Repo, tpl.Ref, dir); err != nil {
			fmt.Printf("[%s] git export failed: %s\n", tpl.Name, err)
			continue
		}

		syncCfg := loadRemoteDefaults(cfg, dir)
		excludes := mergeExcludes(syncCfg)

		paths := syncCfg.Include
		if len(paths) == 0 {
			paths = []string{"."}
		}

		for _, path := range paths {
			src := filepath.Join(dir, path)
			dst := filepath.Join(".", path)

			if err := diffDir(src, dst, excludes, syncCfg.Vars); err != nil {
				fmt.Printf("[%s] diff failed for %s: %s\n", tpl.Name, path, err)
			}
		}
	}
}

func Status(cfg config.Config) {
	lock, err := config.LoadLock()
	if err != nil {
		fmt.Println("error: load lock file:", err)
		return
	}

	if len(lock.Entries) == 0 {
		fmt.Println("no templates have been synced yet")
		return
	}

	for _, tpl := range cfg.Templates {
		entry, ok := lock.Get(tpl.Name)
		if !ok {
			fmt.Printf("[%s] never synced\n", tpl.Name)
			continue
		}

		fmt.Printf("[%s]\n", tpl.Name)
		fmt.Printf("  repo:       %s\n", entry.Repo)
		fmt.Printf("  ref:        %s\n", entry.Ref)
		fmt.Printf("  commit:     %s\n", entry.Commit)
		fmt.Printf("  synced at:  %s\n", entry.Time)

		remote, err := git.RemoteHeadCommit(tpl.Repo, tpl.Ref)
		if err != nil {
			fmt.Printf("  remote:     (failed to check: %s)\n", err)
		} else if remote == entry.Commit {
			fmt.Printf("  status:     up to date\n")
		} else {
			fmt.Printf("  remote:     %s\n", remote)
			fmt.Printf("  status:     update available\n")
		}
		fmt.Println()
	}
}

// dryRunDir shows what would be synced without making changes.
func dryRunDir(src, dst string, excludes []string, vars map[string]string) {
	filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				rel, _ := filepath.Rel(src, path)
				if rel != "." && config.IsExcluded(rel, excludes) {
					return filepath.SkipDir
				}
			}
			return err
		}

		rel, _ := filepath.Rel(src, path)
		if config.IsExcluded(rel, excludes) {
			return nil
		}

		target := filepath.Join(dst, rel)
		srcData, _ := os.ReadFile(path)
		srcData = replaceVars(srcData, vars)
		dstData, dstErr := os.ReadFile(target)

		if dstErr != nil {
			fmt.Printf("  + create: %s\n", rel)
		} else if !bytes.Equal(srcData, dstData) {
			fmt.Printf("  ~ update: %s\n", rel)
		}
		return nil
	})
}

// interactiveSyncDir syncs files with per-file confirmation.
func interactiveSyncDir(src, dst string, excludes []string, vars map[string]string) {
	reader := bufio.NewReader(os.Stdin)

	filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}

		if config.IsExcluded(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		srcData, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		srcData = replaceVars(srcData, vars)

		dstData, dstErr := os.ReadFile(target)
		if dstErr == nil && bytes.Equal(srcData, dstData) {
			return nil // no change
		}

		action := "create"
		if dstErr == nil {
			action = "update"
		}

		fmt.Printf("  %s %s? [y/n/q] ", action, rel)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		switch answer {
		case "y", "yes":
			info, err := d.Info()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			return os.WriteFile(target, srcData, info.Mode())
		case "q", "quit":
			fmt.Println("  aborted")
			return filepath.SkipAll
		default:
			fmt.Println("  skipped")
			return nil
		}
	})
}

// syncDir copies files from src to dst, skipping excluded paths.
func syncDir(src, dst string, excludes []string, vars map[string]string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}

		if config.IsExcluded(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data = replaceVars(data, vars)

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

// diffDir compares files between src (template) and dst (project).
func diffDir(src, dst string, excludes []string, vars map[string]string) error {
	files := make(map[string]bool)

	filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if !config.IsExcluded(rel, excludes) {
			files[rel] = true
		}
		return nil
	})

	filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dst, path)
		if !config.IsExcluded(rel, excludes) {
			files[rel] = true
		}
		return nil
	})

	for rel := range files {
		srcFile := filepath.Join(src, rel)
		dstFile := filepath.Join(dst, rel)

		srcData, srcErr := os.ReadFile(srcFile)
		if srcErr == nil {
			srcData = replaceVars(srcData, vars)
		}
		dstData, dstErr := os.ReadFile(dstFile)

		switch {
		case srcErr != nil && dstErr == nil:
			fmt.Printf("  only in project: %s\n", rel)
		case srcErr == nil && dstErr != nil:
			fmt.Printf("  only in template: %s\n", rel)
		case srcErr == nil && dstErr == nil:
			if !bytes.Equal(srcData, dstData) {
				fmt.Printf("  modified: %s\n", rel)
				printUnifiedDiff(rel, dstData, srcData)
			}
		}
	}

	return nil
}

func printUnifiedDiff(name string, a, b []byte) {
	aLines := strings.Split(string(a), "\n")
	bLines := strings.Split(string(b), "\n")

	fmt.Printf("--- project/%s\n+++ template/%s\n", name, name)

	i, j := 0, 0
	for i < len(aLines) || j < len(bLines) {
		switch {
		case i < len(aLines) && j < len(bLines) && aLines[i] == bLines[j]:
			i++
			j++
		case i < len(aLines) && (j >= len(bLines) || aLines[i] != bLines[j]):
			fmt.Printf("-%s\n", aLines[i])
			i++
		default:
			fmt.Printf("+%s\n", bLines[j])
			j++
		}
	}
	fmt.Println()
}
