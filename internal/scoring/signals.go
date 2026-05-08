package scoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
)

// signalFn is the contract every signal function satisfies. Signals are
// pure functions of the scoring Context and a single Item; they return
// whether the signal fired and a one-line human-readable reason.
type signalFn func(ctx Context, item Item) (triggered bool, reason string)

// signalDef pairs a stable signal name with its weight selector and
// implementation. The name is used as the [domain.ScoreSignal.Name] value
// the agent sees in --explain output and as a config key for weights.
type signalDef struct {
	name   string
	weight func(Weights) int
	fn     signalFn
}

// signalsForIssue is the closed, ordered set of signals evaluated for
// IssueItem values. Order is part of the public contract (stable in
// PriorityScore.Signals); add new signals at the end.
var signalsForIssue = []signalDef{
	{name: "urgent_issue", weight: func(w Weights) int { return w.UrgentIssue }, fn: signalUrgentIssue},
	{name: "overdue_issue", weight: func(w Weights) int { return w.OverdueIssue }, fn: signalOverdueIssue},
	{name: "stale_item", weight: func(w Weights) int { return w.StaleItem }, fn: signalStaleItemForIssue},
}

// signalsForPR is the closed, ordered set of signals evaluated for
// PullRequestItem values.
var signalsForPR = []signalDef{
	{name: "review_requested", weight: func(w Weights) int { return w.ReviewRequested }, fn: signalReviewRequested},
	{name: "blocked_on_user", weight: func(w Weights) int { return w.BlockedOnUser }, fn: signalBlockedOnUser},
	{name: "ci_failing", weight: func(w Weights) int { return w.CIFailing }, fn: signalCIFailing},
	{name: "stale_item", weight: func(w Weights) int { return w.StaleItem }, fn: signalStaleItemForPR},
}

// signalsForEvent is the closed, ordered set of signals evaluated for
// CalendarEventItem values.
var signalsForEvent = []signalDef{
	{name: "meeting_soon", weight: func(w Weights) int { return w.MeetingSoon }, fn: signalMeetingSoon},
	{name: "external_attendees", weight: func(w Weights) int { return w.ExternalAttendees }, fn: signalExternalAttendees},
	{name: "key_stakeholder", weight: func(w Weights) int { return w.KeyStakeholder }, fn: signalKeyStakeholder},
}

// --- signals ---

func signalMeetingSoon(ctx Context, item Item) (bool, string) {
	e := item.(CalendarEventItem).Event
	th := ctx.Thresholds.resolved()
	if !e.StartAt.After(ctx.Now) {
		return false, "already started or ended"
	}
	delta := e.StartAt.Sub(ctx.Now)
	mins := int(delta.Minutes())
	return delta <= th.MeetingSoonWindow, fmt.Sprintf("starts in %d minutes", mins)
}

func signalUrgentIssue(_ Context, item Item) (bool, string) {
	i := item.(IssueItem).Issue
	triggered := i.Priority == domain.PriorityHigh || i.Priority == domain.PriorityUrgent
	return triggered, "priority: " + i.Priority.String()
}

func signalReviewRequested(ctx Context, item Item) (bool, string) {
	pr := item.(PullRequestItem).PR
	if pr.IsBlockedOnUser(ctx.User) {
		return true, "review requested from you"
	}
	return false, "not in requested reviewers"
}

func signalOverdueIssue(ctx Context, item Item) (bool, string) {
	i := item.(IssueItem).Issue
	if i.IsOverdue(ctx.Now) {
		days := int(ctx.Now.Sub(i.DueAt).Hours() / 24)
		return true, fmt.Sprintf("overdue by %d days", days)
	}
	if i.DueAt.IsZero() {
		return false, "no due date"
	}
	if i.State.IsTerminal() {
		return false, "already " + i.State.String()
	}
	days := int(i.DueAt.Sub(ctx.Now).Hours() / 24)
	return false, fmt.Sprintf("due in %d days", days)
}

func signalBlockedOnUser(ctx Context, item Item) (bool, string) {
	pr := item.(PullRequestItem).PR
	if pr.State != domain.PullRequestStateOpen {
		return false, "PR is " + pr.State.String()
	}
	if !pr.IsBlockedOnUser(ctx.User) {
		return false, "not in requested reviewers"
	}
	if pr.CIPassing != nil && !*pr.CIPassing {
		return false, "CI is failing"
	}
	return true, "awaiting your review"
}

func signalStaleItemForIssue(ctx Context, item Item) (bool, string) {
	i := item.(IssueItem).Issue
	return staleResult(ctx, i.UpdatedAt, i.State.IsTerminal())
}

func signalStaleItemForPR(ctx Context, item Item) (bool, string) {
	pr := item.(PullRequestItem).PR
	return staleResult(ctx, pr.UpdatedAt, pr.State.IsTerminal())
}

// staleResult is shared by the issue and PR variants of stale_item.
func staleResult(ctx Context, updatedAt time.Time, terminal bool) (bool, string) {
	if updatedAt.IsZero() {
		return false, "no update timestamp"
	}
	if terminal {
		return false, "already in terminal state"
	}
	th := ctx.Thresholds.resolved()
	since := ctx.Now.Sub(updatedAt)
	days := int(since.Hours() / 24)
	if since > th.StaleAfter {
		return true, fmt.Sprintf("no updates in %d days", days)
	}
	return false, fmt.Sprintf("updated %d days ago", days)
}

func signalCIFailing(_ Context, item Item) (bool, string) {
	pr := item.(PullRequestItem).PR
	if pr.State != domain.PullRequestStateOpen {
		return false, "PR is " + pr.State.String()
	}
	if pr.CIPassing == nil {
		return false, "CI status unknown"
	}
	if *pr.CIPassing {
		return false, "CI checks passing"
	}
	return true, "CI checks failing"
}

func signalExternalAttendees(ctx Context, item Item) (bool, string) {
	e := item.(CalendarEventItem).Event
	if len(e.Attendees) == 0 {
		return false, "no attendees"
	}
	if e.HasExternalAttendees(ctx.User) {
		return true, "has external attendees"
	}
	return false, "all attendees recognized as you"
}

func signalKeyStakeholder(ctx Context, item Item) (bool, string) {
	e := item.(CalendarEventItem).Event
	if e.Organizer == nil {
		return false, "no organizer"
	}
	if len(ctx.KeyStakeholders) == 0 {
		return false, "organizer not in key stakeholders"
	}
	for _, s := range ctx.KeyStakeholders {
		if s == "" {
			continue
		}
		if e.Organizer.Username != "" && s == e.Organizer.Username {
			return true, "organized by key stakeholder " + e.Organizer.String()
		}
		if e.Organizer.Email != "" && strings.EqualFold(s, e.Organizer.Email) {
			return true, "organized by key stakeholder " + e.Organizer.String()
		}
	}
	return false, "organizer not in key stakeholders"
}
