package storage

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// RateSample represents a persisted 5-minute observation window.
type RateSample struct {
	Bucket       time.Time
	OfficialRate decimal.Decimal
	MarketRate   decimal.Decimal
	DeviationPct decimal.Decimal
	NotionalUSDE decimal.Decimal
	CowQuality   string
	CowQuote     json.RawMessage
	BlockNumber  *int64
	Status       string
	Error        *string
	CreatedAt    time.Time
}

// AlertRecord captures an emitted alert for de-duplication/auditing.
type AlertRecord struct {
	ID           int64
	SampleTS     time.Time
	DeviationPct decimal.Decimal
	ThresholdPct decimal.Decimal
	Direction    string
	Channels     []string
	CreatedAt    time.Time
}
