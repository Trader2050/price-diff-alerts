CREATE TABLE rate_samples (
    bucket_ts               timestamptz      PRIMARY KEY,
    official_susde_per_usde NUMERIC(38, 18)  NOT NULL,
    market_susde_per_usde   NUMERIC(38, 18)  NOT NULL,
    deviation_pct           NUMERIC(12, 8)   NOT NULL,
    notional_usde           NUMERIC(38, 18)  NOT NULL,
    cow_quality             TEXT             NOT NULL,
    cow_quote               JSONB            NOT NULL,
    block_number            BIGINT,
    status                  TEXT             NOT NULL DEFAULT 'complete',
    error                   TEXT,
    created_at              timestamptz      NOT NULL DEFAULT now()
);

CREATE INDEX idx_rate_samples_ts_desc ON rate_samples (bucket_ts DESC);
CREATE INDEX idx_rate_samples_deviation ON rate_samples ((abs(deviation_pct)));

CREATE TABLE alerts (
    id            BIGSERIAL       PRIMARY KEY,
    sample_ts     timestamptz     NOT NULL REFERENCES rate_samples(bucket_ts) ON DELETE CASCADE,
    deviation_pct NUMERIC(12, 8)  NOT NULL,
    threshold_pct NUMERIC(12, 8)  NOT NULL,
    direction     TEXT            NOT NULL,
    channels      TEXT[]          NOT NULL,
    created_at    timestamptz     NOT NULL DEFAULT now(),
    UNIQUE(sample_ts)
);

CREATE INDEX idx_alerts_created_at ON alerts (created_at DESC);
