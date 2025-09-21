package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"price-diff-alerts/internal/app"
)

var (
	exportFrom      string
	exportTo        string
	exportPNGPath   string
	exportCSVPath   string
	exportMaxPoints int
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export sampled rates as CSV and/or PNG chart",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := app.ExportOptions{
			PNGPath:   exportPNGPath,
			CSVPath:   exportCSVPath,
			MaxPoints: exportMaxPoints,
		}

		if exportFrom != "" {
			from, err := time.Parse(time.RFC3339, exportFrom)
			if err != nil {
				return fmt.Errorf("invalid --from value: %w", err)
			}
			opts.From = &from
		}

		if exportTo != "" {
			to, err := time.Parse(time.RFC3339, exportTo)
			if err != nil {
				return fmt.Errorf("invalid --to value: %w", err)
			}
			opts.To = &to
		}

		return getApp().Export(cmd.Context(), opts)
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "Start timestamp (RFC3339, inclusive)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "End timestamp (RFC3339, exclusive)")
	exportCmd.Flags().StringVar(&exportPNGPath, "png", "", "Path to write PNG chart")
	exportCmd.Flags().StringVar(&exportCSVPath, "csv", "", "Path to write CSV data")
	exportCmd.Flags().IntVar(&exportMaxPoints, "max-points", 0, "Maximum data points to export (defaults to config)")
}
