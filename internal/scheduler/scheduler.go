package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// TickFunc is invoked on every aligned interval.
type TickFunc func(ctx context.Context, bucket time.Time) error

// Options tune scheduler behaviour.
type Options struct {
	Interval     time.Duration
	AlignToStart bool
	StartupDelay time.Duration
}

// Scheduler drives aligned execution of sampling jobs.
type Scheduler struct {
	opts   Options
	logger zerolog.Logger
}

// New constructs a Scheduler instance.
func New(opts Options, logger zerolog.Logger) *Scheduler {
	if opts.Interval <= 0 {
		panic("scheduler interval must be positive")
	}
	return &Scheduler{opts: opts, logger: logger.With().Str("component", "scheduler").Logger()}
}

// Run blocks, invoking the tick function at each aligned interval until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context, tick TickFunc) error {
	if s.opts.StartupDelay > 0 {
		timer := time.NewTimer(s.opts.StartupDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	next := s.nextTick(time.Now().UTC())
	for {
		delay := time.Until(next)
		if delay < 0 {
			next = s.nextTick(time.Now().UTC())
			delay = time.Until(next)
		}

		timer := time.NewTimer(delay)
		s.logger.Debug().Time("next_bucket", next).Msg("waiting for next bucket")

		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			timer.Stop()
		}

		bucket := s.bucketStart(next)
		s.logger.Info().Time("bucket", bucket).Msg("executing scheduled tick")

		if err := tick(ctx, bucket); err != nil {
			s.logger.Error().Err(err).Time("bucket", bucket).Msg("tick execution failed")
		}

		next = next.Add(s.opts.Interval)
	}
}

func (s *Scheduler) nextTick(now time.Time) time.Time {
	if !s.opts.AlignToStart {
		return now.Add(s.opts.Interval)
	}
	bucket := now.Truncate(s.opts.Interval)
	if !bucket.After(now) {
		bucket = bucket.Add(s.opts.Interval)
	}
	return bucket
}

func (s *Scheduler) bucketStart(t time.Time) time.Time {
	if !s.opts.AlignToStart {
		return t
	}
	return t.Truncate(s.opts.Interval)
}
