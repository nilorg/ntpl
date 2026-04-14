package cmd

import (
	"fmt"

	"github.com/nilorg/ntpl/internal/config"
	"github.com/nilorg/ntpl/internal/sync"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between template and project",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		sync.Diff(cfg)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
