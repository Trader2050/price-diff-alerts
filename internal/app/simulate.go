package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"price-diff-alerts/internal/fetcher"
	"price-diff-alerts/internal/service"
)

// SimulateAlert 通过给定的官方/市场价格模拟一次告警流程。
func (a *App) SimulateAlert(ctx context.Context, official, market decimal.Decimal) error {
	if !a.Config.Alerting.Enabled {
		return errors.New("alerting 未启用")
	}

	notifier := a.newNotifier()
	if notifier == nil {
		return errors.New("未配置任何告警通道")
	}

	off := &staticOfficialFetcher{rate: official}
	mar := &staticMarketFetcher{rate: market}

	svc := service.New(a.Config, nil, off, mar, nil, nil, notifier, a.Logger)

	bucket := time.Now().UTC().Truncate(a.Config.Scheduler.Interval)
	return svc.ProcessBucket(ctx, bucket)
}

type staticOfficialFetcher struct {
	rate decimal.Decimal
}

func (s *staticOfficialFetcher) FetchOfficial(ctx context.Context) (decimal.Decimal, uint64, error) {
	return s.rate, 0, nil
}

type staticMarketFetcher struct {
	rate decimal.Decimal
}

func (s *staticMarketFetcher) FetchMarket(ctx context.Context) (decimal.Decimal, json.RawMessage, string, error) {
	return s.rate, json.RawMessage("{}"), "simulated", nil
}

var _ fetcher.OfficialRateFetcher = (*staticOfficialFetcher)(nil)
var _ fetcher.MarketRateFetcher = (*staticMarketFetcher)(nil)
