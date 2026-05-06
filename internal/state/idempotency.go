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
