package mcpserver

import (
	"errors"
	"fmt"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/scoring"
)

// rfc3339 is the canonical timestamp format used at the wire boundary.
// We accept RFC3339 and RFC3339Nano on input (time.Parse handles both
// when called with RFC3339Nano); we emit RFC3339Nano on output.
const rfc3339 = time.RFC3339Nano

// decodeScoreItem dispatches a [ScoreItem] onto the appropriate
// [scoring.Item] wrapper. Unknown kinds are rejected; missing payload
// fields for the declared kind also produce a clear error.
func decodeScoreItem(in ScoreItem) (scoring.Item, error) {
	switch in.Kind {
	case "issue":
		if in.Issue == nil {
			return nil, errors.New("kind=issue requires an issue payload")
		}
		return scoring.IssueItem{Issue: issueFromPayload(*in.Issue)}, nil
	case "pull_request":
		if in.PullRequest == nil {
			return nil, errors.New("kind=pull_request requires a pull_request payload")
		}
		return scoring.PullRequestItem{PR: prFromPayload(*in.PullRequest)}, nil
	case "calendar_event":
		if in.CalendarEvent == nil {
			return nil, errors.New("kind=calendar_event requires a calendar_event payload")
		}
		return scoring.CalendarEventItem{Event: eventFromPayload(*in.CalendarEvent)}, nil
	default:
		return nil, fmt.Errorf("unknown kind %q (expected issue, pull_request, or calendar_event)", in.Kind)
	}
}

func issueFromPayload(p IssuePayload) domain.Issue {
	return domain.Issue{
		Ref:       refFromPayload(p.Ref),
		Title:     p.Title,
		State:     issueStateFromString(p.State),
		Priority:  priorityFromString(p.Priority),
		DueAt:     parseTimeOrZero(p.DueAt),
		UpdatedAt: parseTimeOrZero(p.UpdatedAt),
	}
}

func prFromPayload(p PRPayload) domain.PullRequest {
	return domain.PullRequest{
		Ref:                refFromPayload(p.Ref),
		Title:              p.Title,
		State:              prStateFromString(p.State),
		RequestedReviewers: peopleFromPayloads(p.RequestedReviewers),
		CIPassing:          p.CIPassing,
		UpdatedAt:          parseTimeOrZero(p.UpdatedAt),
	}
}

func eventFromPayload(p CalEventPayload) domain.CalendarEvent {
	var organizer *domain.Person
	if p.Organizer != nil {
		o := personFromPayload(*p.Organizer)
		organizer = &o
	}
	return domain.CalendarEvent{
		Ref:       refFromPayload(p.Ref),
		Title:     p.Title,
		StartAt:   parseTimeOrZero(p.StartAt),
		EndAt:     parseTimeOrZero(p.EndAt),
		AllDay:    p.AllDay,
		Attendees: peopleFromPayloads(p.Attendees),
		Organizer: organizer,
	}
}

func refFromPayload(p RefPayload) domain.ExternalRef {
	return domain.ExternalRef{Provider: p.Provider, ID: p.ID, URL: p.URL}
}

func personFromPayload(p PersonPayload) domain.Person {
	return domain.Person{Name: p.Name, Email: p.Email, Username: p.Username}
}

func peopleFromPayloads(ps []PersonPayload) []domain.Person {
	out := make([]domain.Person, 0, len(ps))
	for _, p := range ps {
		out = append(out, personFromPayload(p))
	}
	return out
}

// parseTimeOrZero parses an RFC3339(Nano) timestamp, returning the zero
// time.Time for empty input or unparseable values. Unparseable input is
// silently treated as "missing" — the agent's input is best-effort, and
// signals already handle zero times explicitly.
func parseTimeOrZero(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(rfc3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// priorityFromString maps the lowercase priority name to [domain.Priority].
// Unknown values map to [domain.PriorityUnknown].
func priorityFromString(s string) domain.Priority {
	switch s {
	case "low":
		return domain.PriorityLow
	case "medium":
		return domain.PriorityMedium
	case "high":
		return domain.PriorityHigh
	case "urgent":
		return domain.PriorityUrgent
	case "unknown", "":
		return domain.PriorityUnknown
	default:
		return domain.PriorityUnknown
	}
}

// issueStateFromString maps the lowercase state name to [domain.IssueState].
// Unknown values map to [domain.IssueStateUnknown].
func issueStateFromString(s string) domain.IssueState {
	switch s {
	case "backlog":
		return domain.IssueStateBacklog
	case "todo":
		return domain.IssueStateTodo
	case "in_progress":
		return domain.IssueStateInProgress
	case "in_review":
		return domain.IssueStateInReview
	case "done":
		return domain.IssueStateDone
	case "canceled":
		return domain.IssueStateCanceled
	case "unknown", "":
		return domain.IssueStateUnknown
	default:
		return domain.IssueStateUnknown
	}
}

// prStateFromString maps the lowercase state name to [domain.PullRequestState].
// Unknown values map to [domain.PullRequestStateUnknown].
func prStateFromString(s string) domain.PullRequestState {
	switch s {
	case "draft":
		return domain.PullRequestStateDraft
	case "open":
		return domain.PullRequestStateOpen
	case "merged":
		return domain.PullRequestStateMerged
	case "closed":
		return domain.PullRequestStateClosed
	case "unknown", "":
		return domain.PullRequestStateUnknown
	default:
		return domain.PullRequestStateUnknown
	}
}
