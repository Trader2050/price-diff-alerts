package app

import (
	"context"
	"encoding/csv"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
	chart "github.com/wcharczuk/go-chart/v2"

	"price-diff-alerts/internal/storage"
)

// Export renders historical data as CSV and/or PNG.
func (a *App) Export(ctx context.Context, opts ExportOptions) error {
	if opts.CSVPath == "" && opts.PNGPath == "" {
		return errors.New("at least one of --csv or --png must be provided")
	}

	opts.MaxPoints = a.Config.ResolveMaxPoints(opts.MaxPoints)

	store, closeStore, err := a.openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		return errors.New("database not configured; cannot export")
	}
	if closeStore != nil {
		defer closeStore()
	}

	to := time.Now().UTC()
	if opts.To != nil {
		to = opts.To.UTC()
	}

	from := to.Add(-time.Duration(opts.MaxPoints) * a.Config.Scheduler.Interval)
	if opts.From != nil {
		from = opts.From.UTC()
	}

	if !from.Before(to) {
		return errors.New("from must be before to")
	}

	samples, err := store.ListSamplesBetween(ctx, from, to)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		a.Logger.Info().Msg("no samples found for export window")
		return nil
	}

	downsampled := downsampleSamples(samples, opts.MaxPoints)
	a.Logger.Info().Int("total", len(samples)).Int("exported", len(downsampled)).Msg("exporting samples")

	if opts.CSVPath != "" {
		if err := writeSamplesCSV(opts.CSVPath, downsampled); err != nil {
			return err
		}
	}

	if opts.PNGPath != "" {
		if err := writeSamplesPNG(opts.PNGPath, downsampled); err != nil {
			return err
		}
	}

	return nil
}

func downsampleSamples(samples []storage.RateSample, max int) []storage.RateSample {
	if max <= 0 || len(samples) <= max {
		return samples
	}

	result := make([]storage.RateSample, 0, max)
	step := float64(len(samples)-1) / float64(max-1)
	for i := 0; i < max; i++ {
		idx := int(math.Round(step * float64(i)))
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		result = append(result, samples[idx])
	}
	return result
}

func writeSamplesCSV(path string, samples []storage.RateSample) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"bucket_ts", "official_susde_per_usde", "market_susde_per_usde", "deviation_pct", "notional_usde", "cow_quality", "status", "error"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, sample := range samples {
		errMsg := ""
		if sample.Error != nil {
			errMsg = *sample.Error
		}
		record := []string{
			sample.Bucket.Format(time.RFC3339),
			sample.OfficialRate.String(),
			sample.MarketRate.String(),
			sample.DeviationPct.String(),
			sample.NotionalUSDE.String(),
			sample.CowQuality,
			sample.Status,
			errMsg,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return writer.Error()
}

func writeSamplesPNG(path string, samples []storage.RateSample) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	x := make([]time.Time, len(samples))
	official := make([]float64, len(samples))
	market := make([]float64, len(samples))
	deviation := make([]float64, len(samples))

	for i, sample := range samples {
		x[i] = sample.Bucket
		official[i] = sample.OfficialRate.InexactFloat64()
		market[i] = sample.MarketRate.InexactFloat64()
		deviation[i] = sample.DeviationPct.InexactFloat64()
	}

	rateFormatter := func(v interface{}) string {
		return chart.FloatValueFormatterWithFormat(v, "%.3f")
	}
	graph := chart.Chart{
		Width:  1280,
		Height: 720,
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatter,
		},
		YAxis: chart.YAxis{
			Name:           "Rate (sUSDe/USDe)",
			ValueFormatter: rateFormatter,
		},
		YAxisSecondary: chart.YAxis{
			Name:           "Deviation (%)",
			ValueFormatter: rateFormatter,
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Official",
				XValues: x,
				YValues: official,
			},
			chart.TimeSeries{
				Name:    "Market",
				XValues: x,
				YValues: market,
			},
			chart.TimeSeries{
				Name:    "Deviation %",
				XValues: x,
				YValues: deviation,
				YAxis:   chart.YAxisSecondary,
			},
		},
	}
	graph.Elements = []chart.Renderable{chart.Legend(&graph)}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return graph.Render(chart.PNG, file)
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func formatDecimal(d decimal.Decimal, places int32) string {
	return d.StringFixed(places)
}
