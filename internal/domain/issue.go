package domain

import "time"

// IssueState is the canonical lifecycle state of an issue.
type IssueState int

// IssueState values from creation through completion.
const (
	// IssueStateUnknown is the zero value, used when state could not be
	// determined.
	IssueStateUnknown IssueState = iota
	// IssueStateBacklog is filed but not yet planned.
	IssueStateBacklog
	// IssueStateTodo is planned but not started.
	IssueStateTodo
	// IssueStateInProgress is actively being worked on.
	IssueStateInProgress
	// IssueStateInReview is awaiting review.
	IssueStateInReview
	// IssueStateDone is completed successfully.
	IssueStateDone
	// IssueStateCanceled is no longer being pursued.
	IssueStateCanceled
)

// String returns the state's lowercase, snake_case name. Out-of-range
// values return "unknown".
func (s IssueState) String() string {
	switch s {
	case IssueStateBacklog:
		return "backlog"
	case IssueStateTodo:
		return "todo"
	case IssueStateInProgress:
		return "in_progress"
	case IssueStateInReview:
		return "in_review"
	case IssueStateDone:
		return "done"
	case IssueStateCanceled:
		return "canceled"
	case IssueStateUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// IsTerminal reports whether the state means the issue is no longer
// active (Done or Canceled). IssueStateUnknown is not terminal.
func (s IssueState) IsTerminal() bool {
	return s == IssueStateDone || s == IssueStateCanceled
}

// Issue is a work-tracking item: a Linear issue, a Jira ticket, a GitHub
// issue, etc. The provider is identified by Ref.Provider.
type Issue struct {
	// Ref is the provider-agnostic identifier.
	Ref ExternalRef
	// Title is the issue's headline.
	Title string
	// Description is the issue body, if any.
	Description string

	// State is the canonical lifecycle state.
	State IssueState
	// Priority is the canonical priority level.
	Priority Priority

	// Assignee is the person responsible for the issue. Nil if unassigned.
	Assignee *Person
	// Author is the person who created the issue. Nil if unknown.
	Author *Person

	// Labels are user- or provider-applied tags.
	Labels []string

	// ProjectName is the human-readable project or workspace name, if
	// any. Empty string means none. A future Project type will replace
	// this when project relationships are needed.
	ProjectName string

	// CreatedAt is the issue creation time.
	CreatedAt time.Time
	// UpdatedAt is the last modification time.
	UpdatedAt time.Time

	// DueAt is the due date, if set. Zero value means no due date.
	DueAt time.Time
}

// IsOverdue reports whether the issue has a due date in the past relative
// to now AND is not yet in a terminal state. A done or canceled issue is
// never overdue regardless of due date.
func (i Issue) IsOverdue(now time.Time) bool {
	if i.DueAt.IsZero() {
		return false
	}
	if i.State.IsTerminal() {
		return false
	}
	return i.DueAt.Before(now)
}
