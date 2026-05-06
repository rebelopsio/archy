package state

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

func mustRef(provider, id string) domain.ExternalRef {
	return domain.ExternalRef{Provider: provider, ID: id}
}

func TestCarryover_AddThenList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c := Carryover{
		Ref:       mustRef("linear", "LIN-1"),
		Note:      "follow up next week",
		CreatedAt: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
	}
	require.NoError(t, s.CarryoverAdd(ctx, c))

	list, err := s.CarryoverList(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, c.Ref, list[0].Ref)
	assert.Equal(t, "follow up next week", list[0].Note)
	assert.Equal(t, c.CreatedAt, list[0].CreatedAt)
}

func TestCarryover_AddExistingUnresolved_NoOp(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first := Carryover{Ref: mustRef("linear", "LIN-1"), CreatedAt: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)}
	require.NoError(t, s.CarryoverAdd(ctx, first))

	second := Carryover{Ref: mustRef("linear", "LIN-1"), CreatedAt: time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)}
	require.NoError(t, s.CarryoverAdd(ctx, second))

	list, err := s.CarryoverList(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, first.CreatedAt, list[0].CreatedAt, "existing unresolved CreatedAt is preserved")
}

func TestCarryover_AddAfterResolved_CreatesNewRow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ref := mustRef("linear", "LIN-1")
	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: ref, CreatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}))
	require.NoError(t, s.CarryoverMarkResolved(ctx, ref, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)))

	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: ref, CreatedAt: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), Note: "re-flagged"}))

	list, err := s.CarryoverList(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "re-flagged", list[0].Note)
}

func TestCarryover_ListReturnsUnresolvedOldestFirst(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: mustRef("linear", "B"), CreatedAt: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)}))
	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: mustRef("linear", "A"), CreatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}))
	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: mustRef("linear", "C"), CreatedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)}))

	require.NoError(t, s.CarryoverMarkResolved(ctx, mustRef("linear", "B"), time.Now()))

	list, err := s.CarryoverList(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "A", list[0].Ref.ID)
	assert.Equal(t, "C", list[1].Ref.ID)
}

func TestCarryover_MarkResolvedNotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.CarryoverMarkResolved(context.Background(), mustRef("linear", "missing"), time.Now())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestCarryover_MarkResolvedDoesNotAffectAlreadyResolved(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ref := mustRef("linear", "LIN-1")
	require.NoError(t, s.CarryoverAdd(ctx, Carryover{Ref: ref, CreatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}))
	require.NoError(t, s.CarryoverMarkResolved(ctx, ref, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)))

	// Calling MarkResolved again should be ErrNotFound — there's no
	// remaining unresolved row to update.
	err := s.CarryoverMarkResolved(ctx, ref, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}
