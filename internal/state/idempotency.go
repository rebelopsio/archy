package state

import (
	"context"
	"fmt"
	"time"

	"github.com/rebelopsio/archy/internal/state/db"
)

// IdempotencyClaim attempts to claim a unique run identifier. Returns
// (true, nil) when the key is fresh, (false, nil) when already claimed,
// or an error on storage failure.
//
// Claims do not expire automatically; callers choose keys that naturally
// roll over (e.g., "daily-brief:2026-05-06").
func (s *Store) IdempotencyClaim(ctx context.Context, key string, at time.Time) (bool, error) {
	q, err := s.queries()
	if err != nil {
		return false, err
	}
	n, err := q.IdempotencyClaim(ctx, db.IdempotencyClaimParams{
		Key:       key,
		ClaimedAt: formatTime(at),
	})
	if err != nil {
		return false, fmt.Errorf("idempotency claim: %w", err)
	}
	return n == 1, nil
}

// IdempotencyHas reports whether key has been claimed. Pure read with
// no state mutation. Returns (false, nil) when the key is fresh,
// (true, nil) when claimed, or an error on storage failure.
func (s *Store) IdempotencyHas(ctx context.Context, key string) (bool, error) {
	q, err := s.queries()
	if err != nil {
		return false, err
	}
	n, err := q.IdempotencyHas(ctx, key)
	if err != nil {
		return false, fmt.Errorf("idempotency has: %w", err)
	}
	return n == 1, nil
}

// IdempotencyClear removes the claim for key. No-op when the key is
// not claimed. Used by the daily-brief --force flag to discard a prior
// run's claim before regenerating.
func (s *Store) IdempotencyClear(ctx context.Context, key string) error {
	q, err := s.queries()
	if err != nil {
		return err
	}
	if err := q.IdempotencyClear(ctx, key); err != nil {
		return fmt.Errorf("idempotency clear: %w", err)
	}
	return nil
}
