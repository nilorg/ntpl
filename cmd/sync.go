package cmd

import (
	"fmt"

	"github.com/nilorg/ntpl/internal/config"
	"github.com/nilorg/ntpl/internal/sync"

	"github.com/spf13/cobra"
)

var (
	dryRun      bool
	interactive bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync template files to current project",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		sync.Run(cfg, sync.Options{
			DryRun:      dryRun,
			Interactive: interactive,
		})
		ensureGitignore()
	},
}

func init() {
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced without making changes")
	syncCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "confirm each file before syncing")
	rootCmd.AddCommand(syncCmd)
}
