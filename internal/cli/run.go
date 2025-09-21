package cli

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the monitoring service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return getApp().Run(cmd.Context())
	},
}
