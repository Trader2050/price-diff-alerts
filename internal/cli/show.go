package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"price-diff-alerts/internal/app"
)

var (
	showLimit int
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display recent rate samples",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showLimit <= 0 {
			return fmt.Errorf("--limit must be greater than zero")
		}

		opts := app.ShowOptions{
			Limit: showLimit,
		}

		return getApp().Show(cmd.Context(), opts)
	},
}

func init() {
	showCmd.Flags().IntVar(&showLimit, "limit", 20, "Number of samples to display")
}
