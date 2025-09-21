package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"price-diff-alerts/internal/app"
	"price-diff-alerts/internal/config"
	"price-diff-alerts/internal/logging"
)

var (
	cfgFile   string
	logLevel  string
	appHandle *app.App
)

var rootCmd = &cobra.Command{
	Use:   "usdewatcher",
	Short: "Monitor USDe to sUSDe rate deviations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if appHandle != nil {
			return nil
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		if logLevel != "" {
			cfg.Logging.Level = logLevel
		}

		logger := logging.NewLogger(cfg.Logging)
		appHandle = app.NewApp(cfg, logger)
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Override log level defined in config")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(backfillCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(simulateCmd)
}

func getApp() *app.App {
	if appHandle == nil {
		panic("application not initialized; PersistentPreRunE not executed")
	}
	return appHandle
}
