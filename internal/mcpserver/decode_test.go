package mcpserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rebelopsio/archy/internal/domain"
)

func TestPriorityFromString(t *testing.T) {
	cases := map[string]domain.Priority{
		"low":     domain.PriorityLow,
		"medium":  domain.PriorityMedium,
		"high":    domain.PriorityHigh,
		"urgent":  domain.PriorityUrgent,
		"unknown": domain.PriorityUnknown,
		"":        domain.PriorityUnknown,
		"bogus":   domain.PriorityUnknown,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, priorityFromString(in))
		})
	}
}

func TestIssueStateFromString(t *testing.T) {
	cases := map[string]domain.IssueState{
		"backlog":     domain.IssueStateBacklog,
		"todo":        domain.IssueStateTodo,
		"in_progress": domain.IssueStateInProgress,
		"in_review":   domain.IssueStateInReview,
		"done":        domain.IssueStateDone,
		"canceled":    domain.IssueStateCanceled,
		"unknown":     domain.IssueStateUnknown,
		"":            domain.IssueStateUnknown,
		"bogus":       domain.IssueStateUnknown,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, issueStateFromString(in))
		})
	}
}

func TestPRStateFromString(t *testing.T) {
	cases := map[string]domain.PullRequestState{
		"draft":   domain.PullRequestStateDraft,
		"open":    domain.PullRequestStateOpen,
		"merged":  domain.PullRequestStateMerged,
		"closed":  domain.PullRequestStateClosed,
		"unknown": domain.PullRequestStateUnknown,
		"":        domain.PullRequestStateUnknown,
		"bogus":   domain.PullRequestStateUnknown,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, prStateFromString(in))
		})
	}
}

func TestParseTimeOrZero(t *testing.T) {
	t.Run("empty-string-zero", func(t *testing.T) {
		assert.True(t, parseTimeOrZero("").IsZero())
	})
	t.Run("rfc3339", func(t *testing.T) {
		got := parseTimeOrZero("2026-05-06T12:00:00Z")
		assert.Equal(t, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC), got)
	})
	t.Run("rfc3339nano", func(t *testing.T) {
		got := parseTimeOrZero("2026-05-06T12:00:00.123456789Z")
		assert.False(t, got.IsZero())
	})
	t.Run("unparseable-zero", func(t *testing.T) {
		assert.True(t, parseTimeOrZero("not a date").IsZero())
	})
}

func TestDecodeScoreItem_MissingPayload(t *testing.T) {
	cases := []struct {
		name string
		in   ScoreItem
	}{
		{"issue-missing-payload", ScoreItem{Kind: "issue"}},
		{"pr-missing-payload", ScoreItem{Kind: "pull_request"}},
		{"event-missing-payload", ScoreItem{Kind: "calendar_event"}},
		{"unknown-kind", ScoreItem{Kind: "task"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeScoreItem(tc.in)
			assert.Error(t, err)
		})
	}
}
