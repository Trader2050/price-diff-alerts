package app

import (
	"context"
	"errors"
	"time"

	"price-diff-alerts/internal/service"
	"price-diff-alerts/internal/storage"
)

// Backfill processes historical intervals。
func (a *App) Backfill(ctx context.Context, opts BackfillOptions) error {
	interval := a.Config.Scheduler.Interval
	if interval <= 0 {
		return errors.New("scheduler interval 配置不合法")
	}

	start := alignForward(opts.From.UTC(), interval)
	end := opts.To.UTC()
	if !start.Before(end) {
		return errors.New("回填范围为空，请检查 --from/--to")
	}

	if opts.Workers > 1 {
		a.Logger.Warn().Int("workers", opts.Workers).Msg("当前回填暂不支持多线程，将顺序执行")
	}

	var store *storage.Store
	var closeStore func()
	var err error
	var rateStore storage.RateSampleStore

	if opts.DryRun {
		a.Logger.Warn().Msg("回填 dry-run：不会写入数据库")
	} else {
		store, closeStore, err = a.openStore(ctx)
		if err != nil {
			return err
		}
		if store == nil {
			return errors.New("database.dsn 未配置，无法回填")
		}
		if closeStore != nil {
			defer closeStore()
		}
		rateStore = store
	}

	official, market := a.newFetchers()

	svc := service.New(a.Config, nil, official, market, rateStore, nil, nil, a.Logger)

	processed := 0
	failed := 0
	for bucket := start; bucket.Before(end); bucket = bucket.Add(interval) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := svc.ProcessBucket(ctx, bucket); err != nil {
			failed++
			a.Logger.Error().Err(err).Time("bucket", bucket).Msg("回填失败")
			continue
		}
		processed++
	}

	a.Logger.Info().Int("processed", processed).Int("failed", failed).Msg("回填完成")
	if failed > 0 {
		return errors.New("部分 bucket 回填失败，请检查日志")
	}
	return nil
}

func alignForward(t time.Time, interval time.Duration) time.Time {
	truncated := t.Truncate(interval)
	if truncated.Before(t) {
		return truncated.Add(interval)
	}
	return truncated
}
