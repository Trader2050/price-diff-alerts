package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"price-diff-alerts/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "version: %s\ncommit: %s\nbuilt: %s\n", version.Version, version.Commit, version.BuildDate)
	},
}
