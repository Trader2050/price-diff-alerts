package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"price-diff-alerts/internal/alerting"
	"price-diff-alerts/internal/config"
	"price-diff-alerts/internal/fetcher"
	"price-diff-alerts/internal/scheduler"
	"price-diff-alerts/internal/storage"
)

// Service orchestrates fetching, persistence, and alerting.
type Service struct {
	scheduler  *scheduler.Scheduler
	official   fetcher.OfficialRateFetcher
	market     fetcher.MarketRateFetcher
	store      storage.RateSampleStore
	alertStore storage.AlertStore
	notifier   alerting.Notifier
	logger     zerolog.Logger

	threshold decimal.Decimal
	notional  decimal.Decimal
	channels  []string
	alertsOn  bool
	locker    storage.AdvisoryLocker
	lockKey   int64
}

// New constructs the monitoring service.
func New(cfg *config.Config, sched *scheduler.Scheduler, official fetcher.OfficialRateFetcher, market fetcher.MarketRateFetcher, store storage.RateSampleStore, alertStore storage.AlertStore, notifier alerting.Notifier, logger zerolog.Logger) *Service {
	threshold := decimal.Zero
	if cfg.Alerting.Enabled && cfg.Alerting.ThresholdPct > 0 {
		threshold = decimal.NewFromFloat(cfg.Alerting.ThresholdPct)
	}

	notional := decimal.NewFromFloat(cfg.Cow.NotionalUSDE)

	var locker storage.AdvisoryLocker
	if l, ok := store.(storage.AdvisoryLocker); ok {
		locker = l
	}

	return &Service{
		scheduler:  sched,
		official:   official,
		market:     market,
		store:      store,
		alertStore: alertStore,
		notifier:   notifier,
		logger:     logger.With().Str("component", "service").Logger(),
		threshold:  threshold,
		notional:   notional,
		channels:   cfg.Alerting.Channels,
		alertsOn:   cfg.Alerting.Enabled,
		locker:     locker,
		lockKey:    cfg.Scheduler.AdvisoryLockKey,
	}
}

// Run begins the aligned sampling loop.
func (s *Service) Run(ctx context.Context) error {
	if s.scheduler == nil {
		return fmt.Errorf("scheduler not configured")
	}
	return s.scheduler.Run(ctx, s.ProcessBucket)
}

// ProcessBucket 执行单个时间桶的采样逻辑。
func (s *Service) ProcessBucket(ctx context.Context, bucket time.Time) error {
	unlock, proceed, err := s.acquireLock(ctx)
	if err != nil {
		return err
	}
	if !proceed {
		s.logger.Debug().Time("bucket", bucket).Msg("skip bucket because advisory lock held elsewhere")
		return nil
	}
	if unlock != nil {
		defer unlock()
	}

	return s.executeBucket(ctx, bucket)
}

func (s *Service) executeBucket(ctx context.Context, bucket time.Time) error {
	officialRate, blockNumber, err := s.official.FetchOfficial(ctx)
	if err != nil {
		return fmt.Errorf("fetch official rate: %w", err)
	}

	if officialRate.IsZero() {
		return fmt.Errorf("official rate returned zero")
	}

	marketRate, quote, quality, err := s.market.FetchMarket(ctx)
	if err != nil {
		return fmt.Errorf("fetch market rate: %w", err)
	}

	deviation := marketRate.Div(officialRate).Sub(decimal.NewFromInt(1)).Mul(decimal.NewFromInt(100))

	sample := storage.RateSample{
		Bucket:       bucket,
		OfficialRate: officialRate,
		MarketRate:   marketRate,
		DeviationPct: deviation,
		NotionalUSDE: s.notional,
		CowQuality:   quality,
		CowQuote:     quote,
		Status:       "complete",
		CreatedAt:    time.Now().UTC(),
	}
	if blockNumber != 0 {
		block := int64(blockNumber)
		sample.BlockNumber = &block
	}

	if s.store != nil {
		if err := s.store.UpsertRateSample(ctx, sample); err != nil {
			s.logger.Error().Err(err).Time("bucket", bucket).Msg("failed to upsert sample")
		}
	}

	s.logger.Info().Time("bucket", bucket).
		Str("quality", quality).
		Str("deviation_pct", deviation.String()).
		Msg("sample recorded")

	if s.alertsOn && s.notifier != nil && !s.threshold.IsZero() {
		if deviation.Abs().GreaterThan(s.threshold) {
			direction := classifyDeviation(deviation)
			note := alerting.Notification{
				Bucket:       bucket,
				OfficialRate: officialRate,
				MarketRate:   marketRate,
				DeviationPct: deviation,
				ThresholdPct: s.threshold,
				Direction:    direction,
				Channels:     s.channels,
				NotionalUSDE: s.notional,
			}
			if s.alertStore != nil {
				record := storage.AlertRecord{
					SampleTS:     bucket,
					DeviationPct: deviation,
					ThresholdPct: s.threshold,
					Direction:    direction,
					Channels:     s.channels,
				}
				if _, err := s.alertStore.InsertAlert(ctx, record); err != nil {
					s.logger.Error().Err(err).Time("bucket", bucket).Msg("failed to persist alert record")
				}
			}
			if err := s.notifier.Notify(ctx, note); err != nil {
				s.logger.Error().Err(err).Time("bucket", bucket).Msg("failed to dispatch alert")
			}
		}
	}

	return nil
}

func classifyDeviation(d decimal.Decimal) string {
	switch d.Sign() {
	case 1:
		return "up"
	case -1:
		return "down"
	default:
		return "flat"
	}
}

func (s *Service) acquireLock(ctx context.Context) (func(), bool, error) {
	if s.lockKey == 0 || s.locker == nil {
		return nil, true, nil
	}
	unlock, acquired, err := s.locker.TryAdvisoryLock(ctx, s.lockKey)
	if err != nil {
		return nil, false, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		return nil, false, nil
	}
	return unlock, true, nil
}
