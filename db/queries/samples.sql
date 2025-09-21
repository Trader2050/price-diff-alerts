-- name: UpsertRateSample :exec
INSERT INTO rate_samples (
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
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
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
    error                   = EXCLUDED.error;

-- name: ListSamplesBetween :many
SELECT
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
ORDER BY bucket_ts;

-- name: ListRecentSamples :many
SELECT
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
LIMIT $1;

-- name: MarkSampleErrored :exec
UPDATE rate_samples
SET status = 'errored', error = $2
WHERE bucket_ts = $1;

-- name: DeleteSamplesBefore :exec
DELETE FROM rate_samples
WHERE bucket_ts < $1;

-- name: CountSamples :one
SELECT COUNT(*)
FROM rate_samples;
