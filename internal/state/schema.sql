-- archy local state schema. Applied once at Store.Open.
-- Schema changes are additive only: add columns with defaults, never
-- rename or drop. If destructive change becomes necessary, introduce a
-- real migration story at that time.

-- Cache: provider response bodies with TTL.
CREATE TABLE IF NOT EXISTS cache (
    provider    TEXT    NOT NULL,
    key         TEXT    NOT NULL,
    value       BLOB    NOT NULL,
    expires_at  TEXT    NOT NULL,            -- RFC3339Nano UTC
    PRIMARY KEY (provider, key)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS cache_expires_at ON cache(expires_at);

-- Carryover: items flagged for follow-up across days.
CREATE TABLE IF NOT EXISTS carryover (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    provider     TEXT    NOT NULL,
    external_id  TEXT    NOT NULL,
    url          TEXT    NOT NULL DEFAULT '',
    note         TEXT    NOT NULL DEFAULT '',
    created_at   TEXT    NOT NULL,
    resolved_at  TEXT    NOT NULL DEFAULT ''  -- empty string means unresolved
);

CREATE INDEX IF NOT EXISTS carryover_unresolved
    ON carryover(provider, external_id)
    WHERE resolved_at = '';

-- Idempotency: claimed run keys.
CREATE TABLE IF NOT EXISTS idempotency (
    key         TEXT    PRIMARY KEY,
    claimed_at  TEXT    NOT NULL
) WITHOUT ROWID;

-- Explanations: scoring records, one row per (run, item).
CREATE TABLE IF NOT EXISTS explanation (
    run_id       TEXT    NOT NULL,
    provider     TEXT    NOT NULL,
    external_id  TEXT    NOT NULL,
    url          TEXT    NOT NULL DEFAULT '',
    score        INTEGER NOT NULL,
    signals_json TEXT    NOT NULL,            -- JSON []ScoreSignal
    recorded_at  TEXT    NOT NULL,
    PRIMARY KEY (run_id, provider, external_id)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS explanation_by_ref
    ON explanation(provider, external_id, recorded_at DESC);

CREATE INDEX IF NOT EXISTS explanation_by_run
    ON explanation(run_id, recorded_at);
