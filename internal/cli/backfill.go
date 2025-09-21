package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"price-diff-alerts/internal/app"
)

var (
	backfillFrom    string
	backfillTo      string
	backfillDryRun  bool
	backfillWorkers int
)

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Backfill historical samples",
	RunE: func(cmd *cobra.Command, args []string) error {
		if backfillFrom == "" || backfillTo == "" {
			return fmt.Errorf("--from and --to must be provided")
		}

		from, err := time.Parse(time.RFC3339, backfillFrom)
		if err != nil {
			return fmt.Errorf("invalid --from value: %w", err)
		}

		to, err := time.Parse(time.RFC3339, backfillTo)
		if err != nil {
			return fmt.Errorf("invalid --to value: %w", err)
		}

		if !from.Before(to) {
			return fmt.Errorf("--from must be before --to")
		}

		opts := app.BackfillOptions{
			From:    from,
			To:      to,
			DryRun:  backfillDryRun,
			Workers: backfillWorkers,
		}

		return getApp().Backfill(cmd.Context(), opts)
	},
}

func init() {
	backfillCmd.Flags().StringVar(&backfillFrom, "from", "", "Start timestamp (RFC3339, inclusive)")
	backfillCmd.Flags().StringVar(&backfillTo, "to", "", "End timestamp (RFC3339, exclusive)")
	backfillCmd.Flags().BoolVar(&backfillDryRun, "dry-run", false, "Run without writing to storage")
	backfillCmd.Flags().IntVar(&backfillWorkers, "workers", 2, "Number of concurrent workers")
}
