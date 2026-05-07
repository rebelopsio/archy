package linear

import (
	"strings"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
)

// linearIssue is the wire shape of a single issue in Linear's
// list_issues response. Field names match what Linear's MCP server
// returns; verify against the live response if any field appears
// missing in tests. Optional fields use pointer/slice nil to
// distinguish "absent" from "present-and-empty".
type linearIssue struct {
	// ID is Linear's human-readable team-prefixed identifier
	// ("ENG-761"). It is what archy carries in [domain.ExternalRef.ID];
	// archy does not store the underlying UUID.
	ID string `json:"id"`

	// URL is the canonical web URL for the issue.
	URL string `json:"url"`

	// Title is the issue's headline.
	Title string `json:"title"`

	// Description is the issue body, if any.
	Description string `json:"description,omitempty"`

	// Priority is decoded from the Value field; the Name field is
	// ignored. See priorityFromLinear for the mapping.
	Priority *linearPriority `json:"priority,omitempty"`

	// StatusType is the canonical state enum
	// (backlog/unstarted/started/completed/canceled). Decoded from the
	// Linear MCP response's statusType field — NEVER from status, which
	// is the human-readable display name and varies per workspace.
	StatusType string `json:"statusType,omitempty"`

	// Status is the human-readable display name of the state. Not used
	// by archy except for diagnostics; carry through unchanged.
	Status string `json:"status,omitempty"`

	// Assignee is the person the issue is assigned to. Optional.
	Assignee *linearUser `json:"assignee,omitempty"`

	// DueDate is the due date in YYYY-MM-DD form, if set. Empty string
	// means no due date.
	DueDate string `json:"dueDate,omitempty"`

	// CreatedAt and UpdatedAt are RFC3339 timestamps.
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// linearPriority is Linear's priority object: a numeric Value plus a
// human-readable Name. archy decodes from Value only.
type linearPriority struct {
	Value int    `json:"value"`
	Name  string `json:"name,omitempty"`
}

// linearUser is the wire shape of a Linear user (assignee, creator,
// etc.). Linear may not return all fields; populate what's present.
type linearUser struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	// Username may also appear as `handle` or be missing entirely.
	// Try both.
	Username string `json:"username,omitempty"`
	Handle   string `json:"handle,omitempty"`
}

// issueFromLinear converts a Linear-shaped issue to [domain.Issue].
// Provider is hard-coded "linear"; that's the contract.
func issueFromLinear(li linearIssue) domain.Issue {
	return domain.Issue{
		Ref: domain.ExternalRef{
			Provider: "linear",
			ID:       li.ID,
			URL:      li.URL,
		},
		Title:       li.Title,
		Description: li.Description,
		State:       stateFromLinear(li.StatusType),
		Priority:    priorityFromLinear(li.Priority),
		Assignee:    personFromLinear(li.Assignee),
		DueAt:       parseDueDate(li.DueDate),
		CreatedAt:   parseRFC3339(li.CreatedAt),
		UpdatedAt:   parseRFC3339(li.UpdatedAt),
	}
}

// priorityFromLinear maps Linear's numeric priority Value to
// [domain.Priority]. The mapping is inverted from intuition: Linear's
// 1 is the HIGHEST urgency (Urgent) and 4 is the LOWEST (Low). 0
// means "no priority" and maps to the domain's Unknown sentinel.
//
// Reference table:
//
//	Linear Value | Linear Name   | domain.Priority
//	0            | "No priority" | PriorityUnknown
//	1            | "Urgent"      | PriorityUrgent
//	2            | "High"        | PriorityHigh
//	3            | "Medium"      | PriorityMedium
//	4            | "Low"         | PriorityLow
//
// Other values fall back to PriorityUnknown rather than erroring;
// the agent reading the brief sees "unknown" and the user's not
// blocked by Linear's schema growing a new bucket.
func priorityFromLinear(p *linearPriority) domain.Priority {
	if p == nil {
		return domain.PriorityUnknown
	}
	switch p.Value {
	case 1:
		return domain.PriorityUrgent
	case 2:
		return domain.PriorityHigh
	case 3:
		return domain.PriorityMedium
	case 4:
		return domain.PriorityLow
	case 0:
		return domain.PriorityUnknown
	default:
		return domain.PriorityUnknown
	}
}

// stateFromLinear maps Linear's statusType (the canonical enum) to
// [domain.IssueState]. Workspace-specific status names go through
// statusType, not the workspace's display name.
//
// Reference table:
//
//	Linear statusType | domain.IssueState
//	"backlog"         | IssueStateBacklog
//	"unstarted"       | IssueStateTodo
//	"started"         | IssueStateInProgress
//	"completed"       | IssueStateDone
//	"canceled"        | IssueStateCanceled
//	(other)           | IssueStateUnknown
//
// Triaged states (Linear has them) and any custom user-defined
// statusType values default to IssueStateUnknown rather than
// erroring; the brief is best-effort.
func stateFromLinear(statusType string) domain.IssueState {
	switch strings.ToLower(strings.TrimSpace(statusType)) {
	case "backlog":
		return domain.IssueStateBacklog
	case "unstarted":
		return domain.IssueStateTodo
	case "started":
		return domain.IssueStateInProgress
	case "completed":
		return domain.IssueStateDone
	case "canceled", "cancelled":
		return domain.IssueStateCanceled
	default:
		return domain.IssueStateUnknown
	}
}

// personFromLinear converts a Linear user object into a
// [*domain.Person]. Returns nil when the input is nil so the domain
// type's "absent" semantic is preserved.
//
// Linear may name the user handle either "username" or "handle"
// depending on how the MCP server formats responses; we accept both
// and prefer "username" when both are populated.
func personFromLinear(u *linearUser) *domain.Person {
	if u == nil {
		return nil
	}
	username := u.Username
	if username == "" {
		username = u.Handle
	}
	name := u.Name
	if name == "" {
		name = u.DisplayName
	}
	p := &domain.Person{
		Name:     name,
		Email:    u.Email,
		Username: username,
	}
	if p.IsZero() {
		return nil
	}
	return p
}

// parseDueDate parses a YYYY-MM-DD string into a time.Time at midnight
// UTC. Empty string returns the zero time.Time. Unparseable input
// returns the zero time silently — matches the "best-effort" posture
// of the rest of the package.
func parseDueDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	// Some Linear contexts return full RFC3339 here too; try that.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// parseRFC3339 parses an RFC3339(Nano) timestamp. Empty string
// returns the zero time.Time.
func parseRFC3339(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}
