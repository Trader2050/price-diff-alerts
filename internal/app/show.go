package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// Show prints recent samples.
func (a *App) Show(ctx context.Context, opts ShowOptions) error {
	store, closeStore, err := a.openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		return errors.New("database not configured; cannot show samples")
	}
	if closeStore != nil {
		defer closeStore()
	}

	samples, err := store.ListRecentSamples(ctx, opts.Limit)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		fmt.Fprintln(os.Stdout, "no samples found")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "Time (UTC)\tOfficial\tMarket\tDeviation%\tQuality\tStatus\tError")

	for _, sample := range samples {
		errMsg := ""
		if sample.Error != nil {
			errMsg = sanitizeInline(*sample.Error)
		}
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			sample.Bucket.UTC().Format(time.RFC3339),
			formatDecimal(sample.OfficialRate, 3),
			formatDecimal(sample.MarketRate, 3),
			formatDecimal(sample.DeviationPct, 3),
			sample.CowQuality,
			sample.Status,
			errMsg,
		)
	}

	writer.Flush()
	return nil
}

func sanitizeInline(v string) string {
	cleaned := strings.ReplaceAll(v, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")
	return cleaned
}
