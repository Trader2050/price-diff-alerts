package app

import (
	"context"
	"errors"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"price-diff-alerts/internal/alerting"
	"price-diff-alerts/internal/config"
	"price-diff-alerts/internal/fetcher"
	"price-diff-alerts/internal/scheduler"
	"price-diff-alerts/internal/service"
	"price-diff-alerts/internal/storage"
)

// App aggregates configuration and shared dependencies for the CLI commands.
type App struct {
	Config *config.Config
	Logger zerolog.Logger
}

// NewApp constructs a new application handle.
func NewApp(cfg *config.Config, logger zerolog.Logger) *App {
	return &App{Config: cfg, Logger: logger.With().Str("component", "app").Logger()}
}

func (a *App) newFetchers() (fetcher.OfficialRateFetcher, fetcher.MarketRateFetcher) {
	official := fetcher.NewOfficial(fetcher.OfficialOptions{
		RPCURL:       a.Config.Ethereum.RPCURL,
		SUSDEAddress: a.Config.Ethereum.SUSDEAddress,
		Timeout:      a.Config.Ethereum.RequestTimeout,
	}, a.Logger)

	market := fetcher.NewMarket(fetcher.MarketOptions{
		BaseURL:      a.Config.Cow.BaseURL,
		PriceQuality: a.Config.Cow.PriceQuality,
		NotionalUSDE: decimal.NewFromFloat(a.Config.Cow.NotionalUSDE),
		Timeout:      a.Config.Cow.RequestTimeout,
		UserAgent:    a.Config.Cow.UserAgent,
		SellToken:    a.Config.Ethereum.USDEAddress,
		BuyToken:     a.Config.Ethereum.SUSDEAddress,
	}, a.Logger)

	return official, market
}

func (a *App) newNotifier() alerting.Notifier {
	if a.Config.Alerting.Telegram.Enabled {
		cfg := a.Config.Alerting.Telegram
		return alerting.NewTelegramNotifier(cfg.BotToken, cfg.ChatID, cfg.APIBase, 10*time.Second, a.Logger)
	}
	return nil
}

func (a *App) openStore(ctx context.Context) (*storage.Store, func(), error) {
	if a.Config.Database.DSN == "" {
		return nil, nil, nil
	}

	pool, err := storage.NewPool(ctx, a.Config.Database)
	if err != nil {
		return nil, nil, err
	}

	store := storage.NewStore(pool)
	closer := func() {
		store.Close()
	}
	return store, closer, nil
}

// Run executes the long-running monitoring service.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, closeStore, err := a.openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		a.Logger.Warn().Msg("database.dsn not configured; persistence disabled")
	}
	if closeStore != nil {
		defer closeStore()
	}

	sched := scheduler.New(scheduler.Options{
		Interval:     a.Config.Scheduler.Interval,
		AlignToStart: a.Config.Scheduler.AlignToBucket,
		StartupDelay: a.Config.Scheduler.StartupDelay,
	}, a.Logger)

	official, market := a.newFetchers()
	notifier := a.newNotifier()

	var sampleStore storage.RateSampleStore
	var alertStore storage.AlertStore
	if store != nil {
		sampleStore = store
		alertStore = store
	}

	svc := service.New(a.Config, sched, official, market, sampleStore, alertStore, notifier, a.Logger)

	a.Logger.Info().Msg("starting monitoring service")
	err = svc.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		a.Logger.Error().Err(err).Msg("service terminated with error")
		return err
	}

	a.Logger.Info().Msg("monitoring service stopped")
	return nil
}

// ExportOptions hold parameters for exporting historical samples.
type ExportOptions struct {
	From      *time.Time
	To        *time.Time
	PNGPath   string
	CSVPath   string
	MaxPoints int
}

// ShowOptions configure the show command.
type ShowOptions struct {
	Limit int
}

// BackfillOptions configure the backfill job.
type BackfillOptions struct {
	From    time.Time
	To      time.Time
	DryRun  bool
	Workers int
}
