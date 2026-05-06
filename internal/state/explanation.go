package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/state/db"
)

// ExplanationRecord captures one item's scoring outcome on a specific run.
type ExplanationRecord struct {
	// RunID groups records from the same archy invocation.
	RunID string
	// Score is the full PriorityScore, including its signals.
	Score domain.PriorityScore
	// RecordedAt is when the explanation was persisted.
	RecordedAt time.Time
}

// ExplanationPut persists a score's signal breakdown for later
// inspection. Re-recording within the same run (same RunID and ref) is
// idempotent — the existing row is overwritten.
func (s *Store) ExplanationPut(ctx context.Context, runID string, score domain.PriorityScore, at time.Time) error {
	q, err := s.queries()
	if err != nil {
		return err
	}
	signalsJSON, err := json.Marshal(score.Signals)
	if err != nil {
		return fmt.Errorf("explanation put: marshal signals: %w", err)
	}
	if err := q.ExplanationPut(ctx, db.ExplanationPutParams{
		RunID:       runID,
		Provider:    score.Ref.Provider,
		ExternalID:  score.Ref.ID,
		Url:         score.Ref.URL,
		Score:       int64(score.Score),
		SignalsJson: string(signalsJSON),
		RecordedAt:  formatTime(at),
	}); err != nil {
		return fmt.Errorf("explanation put: %w", err)
	}
	return nil
}

// ExplanationGet returns the most recent recorded explanation for ref,
// across all runs. Returns ErrNotFound if no explanation has ever been
// recorded for ref.
func (s *Store) ExplanationGet(ctx context.Context, ref domain.ExternalRef) (ExplanationRecord, error) {
	q, err := s.queries()
	if err != nil {
		return ExplanationRecord{}, err
	}
	row, err := q.ExplanationGet(ctx, db.ExplanationGetParams{
		Provider:   ref.Provider,
		ExternalID: ref.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExplanationRecord{}, fmt.Errorf("%w: ref %s", ErrNotFound, ref.String())
		}
		return ExplanationRecord{}, fmt.Errorf("explanation get: %w", err)
	}
	return explanationFromRow(row)
}

// ExplanationListByRun returns all explanations for a given run, in
// recorded_at order. An unknown run returns an empty slice (no error).
func (s *Store) ExplanationListByRun(ctx context.Context, runID string) ([]ExplanationRecord, error) {
	q, err := s.queries()
	if err != nil {
		return nil, err
	}
	rows, err := q.ExplanationListByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("explanation list by run: %w", err)
	}
	out := make([]ExplanationRecord, 0, len(rows))
	for _, r := range rows {
		rec, err := explanationFromRow(r)
		if err != nil {
			return nil, fmt.Errorf("explanation list by run: %w", err)
		}
		out = append(out, rec)
	}
	return out, nil
}

func explanationFromRow(r db.Explanation) (ExplanationRecord, error) {
	recordedAt, err := parseTime(r.RecordedAt)
	if err != nil {
		return ExplanationRecord{}, fmt.Errorf("parse recorded_at: %w", err)
	}
	var signals []domain.ScoreSignal
	if err := json.Unmarshal([]byte(r.SignalsJson), &signals); err != nil {
		return ExplanationRecord{}, fmt.Errorf("unmarshal signals: %w", err)
	}
	return ExplanationRecord{
		RunID: r.RunID,
		Score: domain.PriorityScore{
			Ref: domain.ExternalRef{
				Provider: r.Provider,
				ID:       r.ExternalID,
				URL:      r.Url,
			},
			Score:      int(r.Score),
			Signals:    signals,
			ComputedAt: recordedAt,
		},
		RecordedAt: recordedAt,
	}, nil
}
