package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nilorg/ntpl/internal/config"

	"github.com/spf13/cobra"
)

var (
	repo  string
	force bool
)

var initCmd = &cobra.Command{
	Use:   "init [repo-url]",
	Short: "Initialize ntpl config in current project",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 && repo == "" {
			repo = args[0]
		}
		if repo == "" {
			fmt.Println("error: repo url is required, use: ntpl init <url> or ntpl init --repo <url>")
			return
		}

		if _, err := os.Stat(".ntpl.yaml"); err == nil && !force {
			fmt.Println("error: .ntpl.yaml already exists, use --force to overwrite")
			return
		}

		cfg := config.Config{
			Templates: []config.Template{
				{
					Name: "default",
					Repo: repo,
					Ref:  "main",
				},
			},
			Sync: config.Sync{
				Include: []string{},
				Exclude: []string{},
			},
		}

		if err := config.Save(cfg); err != nil {
			fmt.Println("error:", err)
			return
		}
		if err := os.MkdirAll(".ntpl", 0755); err != nil {
			fmt.Println("error:", err)
			return
		}

		ensureGitignore()

		fmt.Println("ntpl initialized")
	},
}

// ntplIgnoreEntries are the lines ntpl needs in .gitignore.
var ntplIgnoreEntries = []string{".ntpl/", ".ntpl.lock"}

func ensureGitignore() {
	existing := make(map[string]bool)

	if f, err := os.Open(".gitignore"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
		f.Close()
	}

	var toAdd []string
	for _, entry := range ntplIgnoreEntries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("warning: could not update .gitignore:", err)
		return
	}
	defer f.Close()

	// ensure we start on a new line
	if info, _ := f.Stat(); info.Size() > 0 {
		f.WriteString("\n")
	}
	f.WriteString("# ntpl\n")
	for _, entry := range toAdd {
		f.WriteString(entry + "\n")
	}
	fmt.Println("updated .gitignore")
}

func init() {
	initCmd.Flags().StringVar(&repo, "repo", "", "template repo url")
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite existing .ntpl.yaml")
	rootCmd.AddCommand(initCmd)
}
