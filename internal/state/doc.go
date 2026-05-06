// Package state is archy's local persistence layer.
//
// Backed by SQLite at the path supplied to [Open], it stores four
// independent things:
//
//   - Cache: provider response payloads with TTL so repeated runs
//     within minutes don't re-hit external services.
//   - Carryover: items flagged unresolved in a previous brief, surfaced
//     again until [Store.CarryoverMarkResolved] lands.
//   - Idempotency: claimed run identifiers so re-runs of the same
//     workflow on the same date don't produce duplicate work.
//   - Explanations: scoring inspection records — which signals fired,
//     with what weights, on which run — for --explain and archy doctor.
//
// All times are stored and parsed as RFC3339Nano UTC. The database is
// opened in WAL mode with a single writer connection; archy is not a
// daemon and does not coordinate cross-process access. A missing or
// corrupt database is recoverable: the caller can delete the file and
// reopen with no loss of correctness — caches re-populate, carryover
// starts fresh.
//
// Per ADR-0002, this package imports only [internal/domain], the
// modernc.org/sqlite driver, the sqlc-generated [internal/state/db]
// sub-package, and the standard library.
package state
