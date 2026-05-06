package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rebelopsio/archy/internal/state/db"
)

// CachePut stores raw response bytes under (provider, key) with the
// given TTL, overwriting any existing entry. A zero or negative ttl
// stores an entry that is immediately expired (CacheGet treats it as
// a miss).
func (s *Store) CachePut(ctx context.Context, provider, key string, value []byte, ttl time.Duration) error {
	q, err := s.queries()
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(ttl)
	return q.CachePut(ctx, db.CachePutParams{
		Provider:  provider,
		Key:       key,
		Value:     value,
		ExpiresAt: formatTime(expires),
	})
}

// CacheGet returns the cached value if present and not expired. Returns
// (nil, false, nil) on miss or expiry. Expired entries are not deleted
// here; CacheVacuum handles cleanup.
func (s *Store) CacheGet(ctx context.Context, provider, key string) ([]byte, bool, error) {
	q, err := s.queries()
	if err != nil {
		return nil, false, err
	}
	row, err := q.CacheGet(ctx, db.CacheGetParams{Provider: provider, Key: key})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cache get: %w", err)
	}
	expires, err := parseTime(row.ExpiresAt)
	if err != nil {
		return nil, false, fmt.Errorf("cache get: parse expires_at: %w", err)
	}
	if !expires.After(time.Now().UTC()) {
		return nil, false, nil
	}
	return row.Value, true, nil
}

// CacheVacuum deletes expired cache entries. Returns the number deleted.
func (s *Store) CacheVacuum(ctx context.Context) (int, error) {
	q, err := s.queries()
	if err != nil {
		return 0, err
	}
	n, err := q.CacheVacuum(ctx, formatTime(time.Now().UTC()))
	if err != nil {
		return 0, fmt.Errorf("cache vacuum: %w", err)
	}
	return int(n), nil
}
