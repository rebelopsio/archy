-- Cache --

-- name: CachePut :exec
INSERT INTO cache (provider, key, value, expires_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(provider, key) DO UPDATE SET
    value = excluded.value,
    expires_at = excluded.expires_at;

-- name: CacheGet :one
SELECT value, expires_at FROM cache
WHERE provider = ? AND key = ?;

-- name: CacheVacuum :execrows
DELETE FROM cache WHERE expires_at < ?;

-- Carryover --

-- name: CarryoverFindUnresolved :one
SELECT id, provider, external_id, url, note, created_at, resolved_at
FROM carryover
WHERE provider = ? AND external_id = ? AND resolved_at = ''
ORDER BY id ASC
LIMIT 1;

-- name: CarryoverInsert :exec
INSERT INTO carryover (provider, external_id, url, note, created_at, resolved_at)
VALUES (?, ?, ?, ?, ?, '');

-- name: CarryoverList :many
SELECT id, provider, external_id, url, note, created_at, resolved_at
FROM carryover
WHERE resolved_at = ''
ORDER BY created_at ASC;

-- name: CarryoverMarkResolved :execrows
UPDATE carryover
SET resolved_at = ?
WHERE id = (
    SELECT c.id FROM carryover c
    WHERE c.provider = ? AND c.external_id = ? AND c.resolved_at = ''
    ORDER BY c.id DESC
    LIMIT 1
);

-- Idempotency --

-- name: IdempotencyClaim :execrows
INSERT INTO idempotency (key, claimed_at)
VALUES (?, ?)
ON CONFLICT(key) DO NOTHING;

-- name: IdempotencyHas :one
SELECT EXISTS(SELECT 1 FROM idempotency WHERE key = ?) AS has_claim;

-- name: IdempotencyClear :exec
DELETE FROM idempotency WHERE key = ?;

-- Explanation --

-- name: ExplanationPut :exec
INSERT INTO explanation (run_id, provider, external_id, url, score, signals_json, recorded_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(run_id, provider, external_id) DO UPDATE SET
    url = excluded.url,
    score = excluded.score,
    signals_json = excluded.signals_json,
    recorded_at = excluded.recorded_at;

-- name: ExplanationGet :one
SELECT run_id, provider, external_id, url, score, signals_json, recorded_at
FROM explanation
WHERE provider = ? AND external_id = ?
ORDER BY recorded_at DESC
LIMIT 1;

-- name: ExplanationListByRun :many
SELECT run_id, provider, external_id, url, score, signals_json, recorded_at
FROM explanation
WHERE run_id = ?
ORDER BY recorded_at ASC;
