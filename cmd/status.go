package cmd

import (
	"fmt"

	"github.com/nilorg/ntpl/internal/config"
	"github.com/nilorg/ntpl/internal/sync"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show template sync status",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		sync.Status(cfg)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
