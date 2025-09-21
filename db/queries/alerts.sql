-- name: InsertAlert :one
INSERT INTO alerts (
    sample_ts,
    deviation_pct,
    threshold_pct,
    direction,
    channels
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (sample_ts) DO UPDATE
SET
    deviation_pct = EXCLUDED.deviation_pct,
    threshold_pct = EXCLUDED.threshold_pct,
    direction     = EXCLUDED.direction,
    channels      = EXCLUDED.channels
RETURNING id, sample_ts, deviation_pct, threshold_pct, direction, channels, created_at;

-- name: ListRecentAlerts :many
SELECT
    id,
    sample_ts,
    deviation_pct,
    threshold_pct,
    direction,
    channels,
    created_at
FROM alerts
ORDER BY created_at DESC
LIMIT $1;

-- name: DeleteAlertsBefore :exec
DELETE FROM alerts
WHERE created_at < $1;
