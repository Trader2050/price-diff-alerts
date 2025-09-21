package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	// ErrNotConfigured indicates the storage pool was not initialised.
	ErrNotConfigured = errors.New("storage: pool not configured")
)

const (
	upsertRateSampleSQL = `INSERT INTO rate_samples (
        bucket_ts,
        official_susde_per_usde,
        market_susde_per_usde,
        deviation_pct,
        notional_usde,
        cow_quality,
        cow_quote,
        block_number,
        status,
        error
    ) VALUES (
        $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
    )
    ON CONFLICT (bucket_ts) DO UPDATE
    SET
        official_susde_per_usde = EXCLUDED.official_susde_per_usde,
        market_susde_per_usde   = EXCLUDED.market_susde_per_usde,
        deviation_pct           = EXCLUDED.deviation_pct,
        notional_usde           = EXCLUDED.notional_usde,
        cow_quality             = EXCLUDED.cow_quality,
        cow_quote               = EXCLUDED.cow_quote,
        block_number            = EXCLUDED.block_number,
        status                  = EXCLUDED.status,
        error                   = EXCLUDED.error;`

	listSamplesBetweenSQL = `SELECT
        bucket_ts,
        official_susde_per_usde,
        market_susde_per_usde,
        deviation_pct,
        notional_usde,
        cow_quality,
        cow_quote,
        block_number,
        status,
        error,
        created_at
    FROM rate_samples
    WHERE bucket_ts >= $1
      AND bucket_ts < $2
    ORDER BY bucket_ts;`

	listRecentSamplesSQL = `SELECT
        bucket_ts,
        official_susde_per_usde,
        market_susde_per_usde,
        deviation_pct,
        notional_usde,
        cow_quality,
        cow_quote,
        block_number,
        status,
        error,
        created_at
    FROM rate_samples
    ORDER BY bucket_ts DESC
    LIMIT $1;`

	markSampleErroredSQL = `UPDATE rate_samples
    SET status = 'errored', error = $2
    WHERE bucket_ts = $1;`

	countSamplesSQL = `SELECT COUNT(*) FROM rate_samples;`

	insertAlertSQL = `INSERT INTO alerts (
        sample_ts,
        deviation_pct,
        threshold_pct,
        direction,
        channels
    ) VALUES (
        $1,$2,$3,$4,$5
    )
    ON CONFLICT (sample_ts) DO UPDATE
    SET deviation_pct = EXCLUDED.deviation_pct,
        threshold_pct = EXCLUDED.threshold_pct,
        direction     = EXCLUDED.direction,
        channels      = EXCLUDED.channels
    RETURNING id, sample_ts, deviation_pct, threshold_pct, direction, channels, created_at;`

	listRecentAlertsSQL = `SELECT
        id,
        sample_ts,
        deviation_pct,
        threshold_pct,
        direction,
        channels,
        created_at
    FROM alerts
    ORDER BY created_at DESC
    LIMIT $1;`

	deleteAlertsBeforeSQL = `DELETE FROM alerts WHERE created_at < $1;`

	tryAdvisoryLockSQL = `SELECT pg_try_advisory_lock($1);`
	advisoryUnlockSQL  = `SELECT pg_advisory_unlock($1);`
)

// RateSampleStore defines operations for rate sample persistence.
type RateSampleStore interface {
	UpsertRateSample(ctx context.Context, sample RateSample) error
	ListSamplesBetween(ctx context.Context, from, to time.Time) ([]RateSample, error)
	ListRecentSamples(ctx context.Context, limit int) ([]RateSample, error)
	MarkSampleErrored(ctx context.Context, bucket time.Time, errMsg string) error
	CountSamples(ctx context.Context) (int64, error)
}

// AlertStore defines operations for alert auditing.
type AlertStore interface {
	InsertAlert(ctx context.Context, alert AlertRecord) (AlertRecord, error)
	ListRecentAlerts(ctx context.Context, limit int) ([]AlertRecord, error)
	DeleteAlertsBefore(ctx context.Context, olderThan time.Time) error
}

// AdvisoryLocker exposes advisory lock helpers.
type AdvisoryLocker interface {
	TryAdvisoryLock(ctx context.Context, key int64) (unlock func(), acquired bool, err error)
}

// Store aggregates access to rate samples and alerts.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wires a pgx pool into a Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Close releases the underlying pool resources.
func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

// TryAdvisoryLock attempts to acquire a postgres advisory lock and returns a release func.
func (s *Store) TryAdvisoryLock(ctx context.Context, key int64) (func(), bool, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, false, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire connection: %w", err)
	}

	var acquired bool
	if err := conn.QueryRow(ctx, tryAdvisoryLockSQL, key).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("try advisory lock: %w", err)
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}

	unlock := func() {
		ctxUnlock, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := conn.Exec(ctxUnlock, advisoryUnlockSQL, key); err != nil {
			// unlock best effort; log omitted in storage package
		}
		conn.Release()
	}
	return unlock, true, nil
}

func (s *Store) getPool() (*pgxpool.Pool, error) {
	if s == nil || s.pool == nil {
		return nil, ErrNotConfigured
	}
	return s.pool, nil
}

// UpsertRateSample persists or updates a rate sample.
func (s *Store) UpsertRateSample(ctx context.Context, sample RateSample) error {
	pool, err := s.getPool()
	if err != nil {
		return err
	}

	official := sample.OfficialRate.String()
	market := sample.MarketRate.String()
	deviation := sample.DeviationPct.String()
	notional := sample.NotionalUSDE.String()

	var block interface{}
	if sample.BlockNumber != nil {
		block = *sample.BlockNumber
	}

	var errMsg interface{}
	if sample.Error != nil {
		errMsg = *sample.Error
	}

	_, execErr := pool.Exec(ctx, upsertRateSampleSQL,
		sample.Bucket,
		official,
		market,
		deviation,
		notional,
		sample.CowQuality,
		[]byte(sample.CowQuote),
		block,
		sample.Status,
		errMsg,
	)
	if execErr != nil {
		return fmt.Errorf("upsert rate sample: %w", execErr)
	}
	return nil
}

// ListSamplesBetween lists samples within a time window.
func (s *Store) ListSamplesBetween(ctx context.Context, from, to time.Time) ([]RateSample, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, err
	}

	rows, queryErr := pool.Query(ctx, listSamplesBetweenSQL, from, to)
	if queryErr != nil {
		return nil, fmt.Errorf("list samples between: %w", queryErr)
	}
	defer rows.Close()

	samples := make([]RateSample, 0)
	for rows.Next() {
		sample, scanErr := scanRateSample(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		samples = append(samples, sample)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return samples, nil
}

// ListRecentSamples lists the most recent samples ordered by descending bucket.
func (s *Store) ListRecentSamples(ctx context.Context, limit int) ([]RateSample, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, err
	}

	rows, queryErr := pool.Query(ctx, listRecentSamplesSQL, limit)
	if queryErr != nil {
		return nil, fmt.Errorf("list recent samples: %w", queryErr)
	}
	defer rows.Close()

	samples := make([]RateSample, 0, limit)
	for rows.Next() {
		sample, scanErr := scanRateSample(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		samples = append(samples, sample)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return samples, nil
}

// MarkSampleErrored marks a sample as errored.
func (s *Store) MarkSampleErrored(ctx context.Context, bucket time.Time, errMsg string) error {
	pool, err := s.getPool()
	if err != nil {
		return err
	}
	cmdTag, execErr := pool.Exec(ctx, markSampleErroredSQL, bucket, errMsg)
	if execErr != nil {
		return fmt.Errorf("mark sample errored: %w", execErr)
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CountSamples counts stored samples.
func (s *Store) CountSamples(ctx context.Context) (int64, error) {
	pool, err := s.getPool()
	if err != nil {
		return 0, err
	}
	var count int64
	if scanErr := pool.QueryRow(ctx, countSamplesSQL).Scan(&count); scanErr != nil {
		return 0, fmt.Errorf("count samples: %w", scanErr)
	}
	return count, nil
}

// InsertAlert persists an alert emission.
func (s *Store) InsertAlert(ctx context.Context, alert AlertRecord) (AlertRecord, error) {
	pool, err := s.getPool()
	if err != nil {
		return AlertRecord{}, err
	}

	deviation := alert.DeviationPct.String()
	threshold := alert.ThresholdPct.String()

	row := pool.QueryRow(ctx, insertAlertSQL,
		alert.SampleTS,
		deviation,
		threshold,
		alert.Direction,
		alert.Channels,
	)

	var rec AlertRecord
	var deviationStr, thresholdStr string
	if scanErr := row.Scan(
		&rec.ID,
		&rec.SampleTS,
		&deviationStr,
		&thresholdStr,
		&rec.Direction,
		&rec.Channels,
		&rec.CreatedAt,
	); scanErr != nil {
		return AlertRecord{}, fmt.Errorf("insert alert: %w", scanErr)
	}

	var convErr error
	rec.DeviationPct, convErr = decimal.NewFromString(deviationStr)
	if convErr != nil {
		return AlertRecord{}, fmt.Errorf("parse deviation pct: %w", convErr)
	}
	rec.ThresholdPct, convErr = decimal.NewFromString(thresholdStr)
	if convErr != nil {
		return AlertRecord{}, fmt.Errorf("parse threshold pct: %w", convErr)
	}

	return rec, nil
}

// ListRecentAlerts lists most recent alerts.
func (s *Store) ListRecentAlerts(ctx context.Context, limit int) ([]AlertRecord, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, err
	}

	rows, queryErr := pool.Query(ctx, listRecentAlertsSQL, limit)
	if queryErr != nil {
		return nil, fmt.Errorf("list recent alerts: %w", queryErr)
	}
	defer rows.Close()

	alerts := make([]AlertRecord, 0, limit)
	for rows.Next() {
		var rec AlertRecord
		var deviationStr, thresholdStr string
		if err := rows.Scan(
			&rec.ID,
			&rec.SampleTS,
			&deviationStr,
			&thresholdStr,
			&rec.Direction,
			&rec.Channels,
			&rec.CreatedAt,
		); err != nil {
			return nil, err
		}

		var convErr error
		rec.DeviationPct, convErr = decimal.NewFromString(deviationStr)
		if convErr != nil {
			return nil, fmt.Errorf("parse deviation pct: %w", convErr)
		}
		rec.ThresholdPct, convErr = decimal.NewFromString(thresholdStr)
		if convErr != nil {
			return nil, fmt.Errorf("parse threshold pct: %w", convErr)
		}

		alerts = append(alerts, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return alerts, nil
}

// DeleteAlertsBefore deletes historical alerts.
func (s *Store) DeleteAlertsBefore(ctx context.Context, olderThan time.Time) error {
	pool, err := s.getPool()
	if err != nil {
		return err
	}
	if _, execErr := pool.Exec(ctx, deleteAlertsBeforeSQL, olderThan); execErr != nil {
		return fmt.Errorf("delete alerts before: %w", execErr)
	}
	return nil
}

func scanRateSample(rows pgx.Rows) (RateSample, error) {
	var (
		bucket       time.Time
		officialStr  string
		marketStr    string
		deviationStr string
		notionalStr  string
		cowQuality   string
		cowQuote     json.RawMessage
		block        sql.NullInt64
		status       string
		errMsg       sql.NullString
		createdAt    time.Time
	)

	if err := rows.Scan(
		&bucket,
		&officialStr,
		&marketStr,
		&deviationStr,
		&notionalStr,
		&cowQuality,
		&cowQuote,
		&block,
		&status,
		&errMsg,
		&createdAt,
	); err != nil {
		return RateSample{}, err
	}

	official, err := decimal.NewFromString(officialStr)
	if err != nil {
		return RateSample{}, fmt.Errorf("parse official rate: %w", err)
	}
	market, err := decimal.NewFromString(marketStr)
	if err != nil {
		return RateSample{}, fmt.Errorf("parse market rate: %w", err)
	}
	deviation, err := decimal.NewFromString(deviationStr)
	if err != nil {
		return RateSample{}, fmt.Errorf("parse deviation pct: %w", err)
	}
	notional, err := decimal.NewFromString(notionalStr)
	if err != nil {
		return RateSample{}, fmt.Errorf("parse notional: %w", err)
	}

	sample := RateSample{
		Bucket:       bucket,
		OfficialRate: official,
		MarketRate:   market,
		DeviationPct: deviation,
		NotionalUSDE: notional,
		CowQuality:   cowQuality,
		CowQuote:     cowQuote,
		Status:       status,
		CreatedAt:    createdAt,
	}

	if block.Valid {
		value := block.Int64
		sample.BlockNumber = &value
	}
	if errMsg.Valid {
		msg := errMsg.String
		sample.Error = &msg
	}

	return sample, nil
}
