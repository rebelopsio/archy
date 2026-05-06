package scoring

import "github.com/rebelopsio/archy/internal/domain"

// Item is anything the scoring engine knows how to score. The interface
// is sealed: external packages cannot define their own Item types because
// the applies method takes an unexported sealed value. This keeps the
// signal set closed.
type Item interface {
	// Ref returns the item's external reference. Required for the
	// PriorityScore that the engine produces.
	Ref() domain.ExternalRef

	// applies is unexported so external packages cannot satisfy this
	// interface. It exists only to seal the type set.
	applies(sealed)
}

// sealed is an unexported zero-size type used to close the Item interface.
type sealed struct{}

// IssueItem wraps a domain.Issue for scoring.
type IssueItem struct {
	// Issue is the wrapped issue.
	Issue domain.Issue
}

// Ref returns the wrapped issue's external reference.
func (i IssueItem) Ref() domain.ExternalRef { return i.Issue.Ref }

func (IssueItem) applies(sealed) {}

// PullRequestItem wraps a domain.PullRequest for scoring.
type PullRequestItem struct {
	// PR is the wrapped pull request.
	PR domain.PullRequest
}

// Ref returns the wrapped pull request's external reference.
func (p PullRequestItem) Ref() domain.ExternalRef { return p.PR.Ref }

func (PullRequestItem) applies(sealed) {}

// CalendarEventItem wraps a domain.CalendarEvent for scoring.
type CalendarEventItem struct {
	// Event is the wrapped calendar event.
	Event domain.CalendarEvent
}

// Ref returns the wrapped event's external reference.
func (e CalendarEventItem) Ref() domain.ExternalRef { return e.Event.Ref }

func (CalendarEventItem) applies(sealed) {}
