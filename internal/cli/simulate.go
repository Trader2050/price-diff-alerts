package cli

import (
	"errors"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var (
	simulateOfficial float64
	simulateMarket   float64
)

var simulateCmd = &cobra.Command{
	Use:   "simulate-alert",
	Short: "模拟一次价格偏差并触发告警",
	RunE: func(cmd *cobra.Command, args []string) error {
		if simulateOfficial <= 0 || simulateMarket <= 0 {
			return errors.New("--official 与 --market 必须大于 0")
		}

		official := decimal.NewFromFloat(simulateOfficial)
		market := decimal.NewFromFloat(simulateMarket)
		return getApp().SimulateAlert(cmd.Context(), official, market)
	},
}

func init() {
	simulateCmd.Flags().Float64Var(&simulateOfficial, "official", 0, "官方口径 sUSDe/USDe")
	simulateCmd.Flags().Float64Var(&simulateMarket, "market", 0, "市场口径 sUSDe/USDe")
}
