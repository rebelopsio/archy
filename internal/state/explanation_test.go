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

func sampleScore(refID string) domain.PriorityScore {
	return domain.PriorityScore{
		Ref:   domain.ExternalRef{Provider: "linear", ID: refID, URL: "https://example.com/" + refID},
		Score: 15,
		Signals: []domain.ScoreSignal{
			{Name: "urgent_issue", Weight: 8, Triggered: true, Reason: "priority: urgent"},
			{Name: "stale_item", Weight: 2, Triggered: false, Reason: "no updates in 1 days"},
			{Name: "overdue_issue", Weight: 5, Triggered: true, Reason: "overdue by 2 days"},
		},
		ComputedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
}

func TestExplanation_PutThenGet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	score := sampleScore("LIN-1")
	at := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.ExplanationPut(ctx, "run-1", score, at))

	rec, err := s.ExplanationGet(ctx, score.Ref)
	require.NoError(t, err)
	assert.Equal(t, "run-1", rec.RunID)
	assert.Equal(t, score.Ref, rec.Score.Ref)
	assert.Equal(t, score.Score, rec.Score.Score)
	assert.Equal(t, score.Signals, rec.Score.Signals)
}

func TestExplanation_GetUnknownRefReturnsErrNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ExplanationGet(context.Background(), domain.ExternalRef{Provider: "linear", ID: "missing"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestExplanation_PutSameRunOverwrites(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ref := domain.ExternalRef{Provider: "linear", ID: "LIN-1"}
	at := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	require.NoError(t, s.ExplanationPut(ctx, "run-1", domain.PriorityScore{Ref: ref, Score: 5}, at))
	require.NoError(t, s.ExplanationPut(ctx, "run-1", domain.PriorityScore{Ref: ref, Score: 10}, at))

	rec, err := s.ExplanationGet(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, 10, rec.Score.Score)
}

func TestExplanation_GetReturnsMostRecentAcrossRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ref := domain.ExternalRef{Provider: "linear", ID: "LIN-1"}

	require.NoError(t, s.ExplanationPut(ctx, "run-1", domain.PriorityScore{Ref: ref, Score: 5}, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)))
	require.NoError(t, s.ExplanationPut(ctx, "run-2", domain.PriorityScore{Ref: ref, Score: 10}, time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)))

	rec, err := s.ExplanationGet(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "run-2", rec.RunID)
	assert.Equal(t, 10, rec.Score.Score)
}

func TestExplanation_ListByRunReturnsAllInOrder(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.ExplanationPut(ctx, "run-1", sampleScore("A"), time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)))
	require.NoError(t, s.ExplanationPut(ctx, "run-1", sampleScore("B"), time.Date(2026, 5, 6, 12, 1, 0, 0, time.UTC)))
	require.NoError(t, s.ExplanationPut(ctx, "run-1", sampleScore("C"), time.Date(2026, 5, 6, 12, 2, 0, 0, time.UTC)))

	list, err := s.ExplanationListByRun(ctx, "run-1")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "A", list[0].Score.Ref.ID)
	assert.Equal(t, "B", list[1].Score.Ref.ID)
	assert.Equal(t, "C", list[2].Score.Ref.ID)
}

func TestExplanation_ListByRunUnknown(t *testing.T) {
	s := openTestStore(t)
	list, err := s.ExplanationListByRun(context.Background(), "no-such-run")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestExplanation_RoundTripSignals(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	original := domain.PriorityScore{
		Ref: domain.ExternalRef{Provider: "linear", ID: "LIN-1"},
		Signals: []domain.ScoreSignal{
			{Name: "urgent_issue", Weight: 8, Triggered: true, Reason: "priority: urgent"},
			{Name: "stale_item", Weight: 0, Triggered: false, Reason: "the system observed no update timestamp on this item, which is a long-form reason intended to verify json round-tripping handles arbitrary string lengths without truncation"},
		},
		ComputedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, s.ExplanationPut(ctx, "run-1", original, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)))

	rec, err := s.ExplanationGet(ctx, original.Ref)
	require.NoError(t, err)
	assert.Equal(t, original.Signals, rec.Score.Signals)
}
