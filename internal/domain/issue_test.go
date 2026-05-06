package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIssueState_String(t *testing.T) {
	cases := []struct {
		name string
		s    IssueState
		want string
	}{
		{"unknown", IssueStateUnknown, "unknown"},
		{"backlog", IssueStateBacklog, "backlog"},
		{"todo", IssueStateTodo, "todo"},
		{"in_progress", IssueStateInProgress, "in_progress"},
		{"in_review", IssueStateInReview, "in_review"},
		{"done", IssueStateDone, "done"},
		{"canceled", IssueStateCanceled, "canceled"},
		{"out-of-range", IssueState(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.s.String())
		})
	}
}

func TestIssueState_IsTerminal(t *testing.T) {
	cases := []struct {
		name string
		s    IssueState
		want bool
	}{
		{"unknown", IssueStateUnknown, false},
		{"backlog", IssueStateBacklog, false},
		{"todo", IssueStateTodo, false},
		{"in_progress", IssueStateInProgress, false},
		{"in_review", IssueStateInReview, false},
		{"done", IssueStateDone, true},
		{"canceled", IssueStateCanceled, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.s.IsTerminal())
		})
	}
}

func TestIssue_IsOverdue(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	cases := []struct {
		name  string
		issue Issue
		want  bool
	}{
		{
			name:  "past-due-and-active",
			issue: Issue{State: IssueStateInProgress, DueAt: past},
			want:  true,
		},
		{
			name:  "future-due",
			issue: Issue{State: IssueStateInProgress, DueAt: future},
			want:  false,
		},
		{
			name:  "no-due-date",
			issue: Issue{State: IssueStateInProgress},
			want:  false,
		},
		{
			name:  "past-due-but-done",
			issue: Issue{State: IssueStateDone, DueAt: past},
			want:  false,
		},
		{
			name:  "past-due-but-canceled",
			issue: Issue{State: IssueStateCanceled, DueAt: past},
			want:  false,
		},
		{
			name:  "due-equals-now",
			issue: Issue{State: IssueStateTodo, DueAt: now},
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.issue.IsOverdue(now))
		})
	}
}
