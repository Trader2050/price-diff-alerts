package fetcher

import (
	"context"
	"encoding/json"

	"github.com/shopspring/decimal"
)

// OfficialRateFetcher retrieves the on-chain official sUSDe/USDe rate.
type OfficialRateFetcher interface {
	FetchOfficial(ctx context.Context) (decimal.Decimal, uint64, error)
}

// MarketRateFetcher retrieves the secondary market rate from CoW Protocol.
type MarketRateFetcher interface {
	FetchMarket(ctx context.Context) (decimal.Decimal, json.RawMessage, string, error)
}
