package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rebelopsio/archy/internal/domain"
)

// fixedNow is the reference time every signal test uses, so dates in
// fixtures are predictable.
var fixedNow = time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

func baseCtx() Context {
	return Context{
		Now:  fixedNow,
		User: domain.MakeIdentity([]string{"user@example.com"}, "user", "user"),
	}
}

// --- meeting_soon ---

func TestSignal_MeetingSoon_TriggeredWithin10m(t *testing.T) {
	ctx := baseCtx()
	e := domain.CalendarEvent{StartAt: fixedNow.Add(10 * time.Minute), EndAt: fixedNow.Add(40 * time.Minute)}
	triggered, reason := signalMeetingSoon(ctx, CalendarEventItem{Event: e})
	assert.True(t, triggered)
	assert.Equal(t, "starts in 10 minutes", reason)
}

func TestSignal_MeetingSoon_NotTriggeredFarFuture(t *testing.T) {
	ctx := baseCtx()
	e := domain.CalendarEvent{StartAt: fixedNow.Add(2 * time.Hour)}
	triggered, reason := signalMeetingSoon(ctx, CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "starts in 120 minutes", reason)
}

func TestSignal_MeetingSoon_NotTriggeredAlreadyStarted(t *testing.T) {
	ctx := baseCtx()
	e := domain.CalendarEvent{StartAt: fixedNow.Add(-time.Minute)}
	triggered, reason := signalMeetingSoon(ctx, CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "already started or ended", reason)
}

// --- urgent_issue ---

func TestSignal_UrgentIssue(t *testing.T) {
	cases := []struct {
		name     string
		priority domain.Priority
		want     bool
		reason   string
	}{
		{"high", domain.PriorityHigh, true, "priority: high"},
		{"urgent", domain.PriorityUrgent, true, "priority: urgent"},
		{"medium", domain.PriorityMedium, false, "priority: medium"},
		{"low", domain.PriorityLow, false, "priority: low"},
		{"unknown", domain.PriorityUnknown, false, "priority: unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := domain.Issue{Priority: tc.priority}
			triggered, reason := signalUrgentIssue(baseCtx(), IssueItem{Issue: i})
			assert.Equal(t, tc.want, triggered)
			assert.Equal(t, tc.reason, reason)
		})
	}
}

// --- review_requested ---

func TestSignal_ReviewRequested_UsernameMatch(t *testing.T) {
	pr := domain.PullRequest{RequestedReviewers: []domain.Person{{Username: "user"}}}
	triggered, reason := signalReviewRequested(baseCtx(), PullRequestItem{PR: pr})
	assert.True(t, triggered)
	assert.Equal(t, "review requested from you", reason)
}

func TestSignal_ReviewRequested_EmailMatchWhenNoUsername(t *testing.T) {
	pr := domain.PullRequest{RequestedReviewers: []domain.Person{{Email: "user@example.com"}}}
	ctx := baseCtx()
	// Reviewer has only email; identity matches via email-set membership.
	triggered, reason := signalReviewRequested(ctx, PullRequestItem{PR: pr})
	assert.True(t, triggered)
	assert.Equal(t, "review requested from you", reason)
}

func TestSignal_ReviewRequested_NotMatched(t *testing.T) {
	pr := domain.PullRequest{RequestedReviewers: []domain.Person{{Username: "alice"}}}
	triggered, reason := signalReviewRequested(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "not in requested reviewers", reason)
}

func TestSignal_ReviewRequested_EmptyReviewers(t *testing.T) {
	triggered, reason := signalReviewRequested(baseCtx(), PullRequestItem{PR: domain.PullRequest{}})
	assert.False(t, triggered)
	assert.Equal(t, "not in requested reviewers", reason)
}

// --- overdue_issue ---

func TestSignal_OverdueIssue_TriggeredPastDue(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateInProgress, DueAt: fixedNow.Add(-3 * 24 * time.Hour)}
	triggered, reason := signalOverdueIssue(baseCtx(), IssueItem{Issue: i})
	assert.True(t, triggered)
	assert.Equal(t, "overdue by 3 days", reason)
}

func TestSignal_OverdueIssue_NotTriggeredZeroDueAt(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateInProgress}
	triggered, reason := signalOverdueIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "no due date", reason)
}

func TestSignal_OverdueIssue_NotTriggeredWhenDone(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateDone, DueAt: fixedNow.Add(-3 * 24 * time.Hour)}
	triggered, reason := signalOverdueIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "already done", reason)
}

func TestSignal_OverdueIssue_NotTriggeredFutureDue(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateInProgress, DueAt: fixedNow.Add(5 * 24 * time.Hour)}
	triggered, reason := signalOverdueIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "due in 5 days", reason)
}

// --- blocked_on_user ---

func TestSignal_BlockedOnUser_TriggeredCIPassing(t *testing.T) {
	pass := true
	pr := domain.PullRequest{
		State:              domain.PullRequestStateOpen,
		RequestedReviewers: []domain.Person{{Username: "user"}},
		CIPassing:          &pass,
	}
	triggered, reason := signalBlockedOnUser(baseCtx(), PullRequestItem{PR: pr})
	assert.True(t, triggered)
	assert.Equal(t, "awaiting your review", reason)
}

func TestSignal_BlockedOnUser_TriggeredCIUnknown(t *testing.T) {
	pr := domain.PullRequest{
		State:              domain.PullRequestStateOpen,
		RequestedReviewers: []domain.Person{{Username: "user"}},
	}
	triggered, _ := signalBlockedOnUser(baseCtx(), PullRequestItem{PR: pr})
	assert.True(t, triggered)
}

func TestSignal_BlockedOnUser_NotTriggeredCIFailing(t *testing.T) {
	fail := false
	pr := domain.PullRequest{
		State:              domain.PullRequestStateOpen,
		RequestedReviewers: []domain.Person{{Username: "user"}},
		CIPassing:          &fail,
	}
	triggered, reason := signalBlockedOnUser(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "CI is failing", reason)
}

func TestSignal_BlockedOnUser_NotTriggeredMerged(t *testing.T) {
	pr := domain.PullRequest{State: domain.PullRequestStateMerged}
	triggered, reason := signalBlockedOnUser(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "PR is merged", reason)
}

func TestSignal_BlockedOnUser_NotTriggeredNotInReviewers(t *testing.T) {
	pr := domain.PullRequest{State: domain.PullRequestStateOpen}
	triggered, reason := signalBlockedOnUser(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "not in requested reviewers", reason)
}

// --- stale_item (issue + PR) ---

func TestSignal_StaleItem_Issue_TriggeredOld(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateTodo, UpdatedAt: fixedNow.Add(-30 * 24 * time.Hour)}
	triggered, reason := signalStaleItemForIssue(baseCtx(), IssueItem{Issue: i})
	assert.True(t, triggered)
	assert.Equal(t, "no updates in 30 days", reason)
}

func TestSignal_StaleItem_Issue_NotTriggeredTerminal(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateDone, UpdatedAt: fixedNow.Add(-30 * 24 * time.Hour)}
	triggered, reason := signalStaleItemForIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "already in terminal state", reason)
}

func TestSignal_StaleItem_Issue_NotTriggeredZeroUpdate(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateTodo}
	triggered, reason := signalStaleItemForIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "no update timestamp", reason)
}

func TestSignal_StaleItem_Issue_NotTriggeredRecent(t *testing.T) {
	i := domain.Issue{State: domain.IssueStateTodo, UpdatedAt: fixedNow.Add(-2 * 24 * time.Hour)}
	triggered, reason := signalStaleItemForIssue(baseCtx(), IssueItem{Issue: i})
	assert.False(t, triggered)
	assert.Equal(t, "updated 2 days ago", reason)
}

func TestSignal_StaleItem_PR_TriggeredOld(t *testing.T) {
	pr := domain.PullRequest{State: domain.PullRequestStateOpen, UpdatedAt: fixedNow.Add(-30 * 24 * time.Hour)}
	triggered, reason := signalStaleItemForPR(baseCtx(), PullRequestItem{PR: pr})
	assert.True(t, triggered)
	assert.Equal(t, "no updates in 30 days", reason)
}

func TestSignal_StaleItem_PR_NotTriggeredMerged(t *testing.T) {
	pr := domain.PullRequest{State: domain.PullRequestStateMerged, UpdatedAt: fixedNow.Add(-30 * 24 * time.Hour)}
	triggered, _ := signalStaleItemForPR(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
}

// --- ci_failing ---

func TestSignal_CIFailing_Triggered(t *testing.T) {
	fail := false
	pr := domain.PullRequest{State: domain.PullRequestStateOpen, CIPassing: &fail}
	triggered, reason := signalCIFailing(baseCtx(), PullRequestItem{PR: pr})
	assert.True(t, triggered)
	assert.Equal(t, "CI checks failing", reason)
}

func TestSignal_CIFailing_NotTriggeredCIPassing(t *testing.T) {
	pass := true
	pr := domain.PullRequest{State: domain.PullRequestStateOpen, CIPassing: &pass}
	triggered, reason := signalCIFailing(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "CI checks passing", reason)
}

func TestSignal_CIFailing_NotTriggeredCINil(t *testing.T) {
	pr := domain.PullRequest{State: domain.PullRequestStateOpen}
	triggered, reason := signalCIFailing(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "CI status unknown", reason)
}

func TestSignal_CIFailing_NotTriggeredMerged(t *testing.T) {
	fail := false
	pr := domain.PullRequest{State: domain.PullRequestStateMerged, CIPassing: &fail}
	triggered, reason := signalCIFailing(baseCtx(), PullRequestItem{PR: pr})
	assert.False(t, triggered)
	assert.Equal(t, "PR is merged", reason)
}

// --- external_attendees ---

func TestSignal_ExternalAttendees_Triggered(t *testing.T) {
	e := domain.CalendarEvent{Attendees: []domain.Person{
		{Email: "user@example.com"},
		{Email: "vendor@external.com"},
	}}
	triggered, reason := signalExternalAttendees(baseCtx(), CalendarEventItem{Event: e})
	assert.True(t, triggered)
	assert.Equal(t, "has external attendees", reason)
}

func TestSignal_ExternalAttendees_NotTriggeredAllAreOperator(t *testing.T) {
	e := domain.CalendarEvent{Attendees: []domain.Person{
		{Email: "user@example.com"},
		{Email: "alt@personal.io"},
	}}
	ctx := baseCtx()
	ctx.User = domain.MakeIdentity([]string{"user@example.com", "alt@personal.io"}, "", "")
	triggered, reason := signalExternalAttendees(ctx, CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "all attendees recognized as you", reason)
}

func TestSignal_ExternalAttendees_NotTriggeredNoAttendees(t *testing.T) {
	e := domain.CalendarEvent{}
	triggered, reason := signalExternalAttendees(baseCtx(), CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "no attendees", reason)
}

func TestSignal_ExternalAttendees_NotTriggeredEmptyIdentity(t *testing.T) {
	// With no emails configured on the operator, the signal cannot
	// classify anyone as external — the predicate short-circuits to false.
	e := domain.CalendarEvent{Attendees: []domain.Person{{Email: "alice@example.com"}}}
	ctx := baseCtx()
	ctx.User = domain.MakeIdentity(nil, "", "")
	triggered, reason := signalExternalAttendees(ctx, CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "all attendees recognized as you", reason)
}

// --- key_stakeholder ---

func TestSignal_KeyStakeholder_TriggeredByUsername(t *testing.T) {
	e := domain.CalendarEvent{Organizer: &domain.Person{Username: "ceo", Name: "CEO"}}
	ctx := baseCtx()
	ctx.KeyStakeholders = []string{"ceo"}
	triggered, reason := signalKeyStakeholder(ctx, CalendarEventItem{Event: e})
	assert.True(t, triggered)
	assert.Contains(t, reason, "key stakeholder")
}

func TestSignal_KeyStakeholder_TriggeredByEmailCaseInsensitive(t *testing.T) {
	e := domain.CalendarEvent{Organizer: &domain.Person{Email: "vp@example.com", Name: "VP"}}
	ctx := baseCtx()
	ctx.KeyStakeholders = []string{"VP@EXAMPLE.COM"}
	triggered, reason := signalKeyStakeholder(ctx, CalendarEventItem{Event: e})
	assert.True(t, triggered)
	assert.Contains(t, reason, "key stakeholder")
}

func TestSignal_KeyStakeholder_NotTriggeredEmptyList(t *testing.T) {
	e := domain.CalendarEvent{Organizer: &domain.Person{Username: "ceo"}}
	triggered, reason := signalKeyStakeholder(baseCtx(), CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "organizer not in key stakeholders", reason)
}

func TestSignal_KeyStakeholder_NotTriggeredNoOrganizer(t *testing.T) {
	e := domain.CalendarEvent{}
	ctx := baseCtx()
	ctx.KeyStakeholders = []string{"ceo"}
	triggered, reason := signalKeyStakeholder(ctx, CalendarEventItem{Event: e})
	assert.False(t, triggered)
	assert.Equal(t, "no organizer", reason)
}
