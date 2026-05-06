package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/state/db"
)

// Carryover represents an item flagged for follow-up across days.
type Carryover struct {
	// Ref is the provider-agnostic identifier of the carried-over item.
	Ref domain.ExternalRef
	// Note is a free-form description of why the item was carried over.
	Note string
	// CreatedAt is when the carryover was first added.
	CreatedAt time.Time
	// ResolvedAt is when the carryover was resolved, or the zero value
	// for unresolved carryovers.
	ResolvedAt time.Time
}

// CarryoverAdd records a follow-up. If a Carryover with the same Ref
// already exists and is unresolved, this is a no-op (the existing
// CreatedAt is preserved). If the existing entry was resolved, a new
// row is created — re-flagging is meaningful.
func (s *Store) CarryoverAdd(ctx context.Context, c Carryover) error {
	q, err := s.queries()
	if err != nil {
		return err
	}
	// Check for an existing unresolved row.
	_, err = q.CarryoverFindUnresolved(ctx, db.CarryoverFindUnresolvedParams{
		Provider:   c.Ref.Provider,
		ExternalID: c.Ref.ID,
	})
	switch {
	case err == nil:
		return nil // existing unresolved row; no-op
	case errors.Is(err, sql.ErrNoRows):
		// fall through to insert
	default:
		return fmt.Errorf("carryover add: lookup: %w", err)
	}
	created := c.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	if err := q.CarryoverInsert(ctx, db.CarryoverInsertParams{
		Provider:   c.Ref.Provider,
		ExternalID: c.Ref.ID,
		Url:        c.Ref.URL,
		Note:       c.Note,
		CreatedAt:  formatTime(created),
	}); err != nil {
		return fmt.Errorf("carryover add: insert: %w", err)
	}
	return nil
}

// CarryoverList returns all unresolved carryovers, oldest first.
func (s *Store) CarryoverList(ctx context.Context) ([]Carryover, error) {
	q, err := s.queries()
	if err != nil {
		return nil, err
	}
	rows, err := q.CarryoverList(ctx)
	if err != nil {
		return nil, fmt.Errorf("carryover list: %w", err)
	}
	out := make([]Carryover, 0, len(rows))
	for _, r := range rows {
		c, err := carryoverFromRow(r)
		if err != nil {
			return nil, fmt.Errorf("carryover list: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// CarryoverMarkResolved marks the most recent unresolved carryover
// matching ref as resolved at the given time. Returns ErrNotFound if no
// unresolved carryover exists.
func (s *Store) CarryoverMarkResolved(ctx context.Context, ref domain.ExternalRef, at time.Time) error {
	q, err := s.queries()
	if err != nil {
		return err
	}
	n, err := q.CarryoverMarkResolved(ctx, db.CarryoverMarkResolvedParams{
		ResolvedAt: formatTime(at),
		Provider:   ref.Provider,
		ExternalID: ref.ID,
	})
	if err != nil {
		return fmt.Errorf("carryover mark resolved: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: ref %s", ErrNotFound, ref.String())
	}
	return nil
}

func carryoverFromRow(r db.Carryover) (Carryover, error) {
	created, err := parseTime(r.CreatedAt)
	if err != nil {
		return Carryover{}, fmt.Errorf("parse created_at: %w", err)
	}
	resolved, err := parseTime(r.ResolvedAt)
	if err != nil {
		return Carryover{}, fmt.Errorf("parse resolved_at: %w", err)
	}
	return Carryover{
		Ref: domain.ExternalRef{
			Provider: r.Provider,
			ID:       r.ExternalID,
			URL:      r.Url,
		},
		Note:       r.Note,
		CreatedAt:  created,
		ResolvedAt: resolved,
	}, nil
}
