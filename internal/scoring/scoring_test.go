package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

func TestScore_IssueSignalsInStableOrder(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{UrgentIssue: 8, OverdueIssue: 5, StaleItem: 2}
	i := domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "LIN-1"}, Priority: domain.PriorityUrgent}
	score := Score(ctx, IssueItem{Issue: i})

	require.Len(t, score.Signals, 3)
	assert.Equal(t, "urgent_issue", score.Signals[0].Name)
	assert.Equal(t, "overdue_issue", score.Signals[1].Name)
	assert.Equal(t, "stale_item", score.Signals[2].Name)
}

func TestScore_PRSignalsInStableOrder(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{ReviewRequested: 7, BlockedOnUser: 6, CIFailing: 4, StaleItem: 2}
	pr := domain.PullRequest{Ref: domain.ExternalRef{Provider: "github", ID: "1"}, State: domain.PullRequestStateOpen}
	score := Score(ctx, PullRequestItem{PR: pr})

	require.Len(t, score.Signals, 4)
	assert.Equal(t, "review_requested", score.Signals[0].Name)
	assert.Equal(t, "blocked_on_user", score.Signals[1].Name)
	assert.Equal(t, "ci_failing", score.Signals[2].Name)
	assert.Equal(t, "stale_item", score.Signals[3].Name)
}

func TestScore_EventSignalsInStableOrder(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{MeetingSoon: 5, ExternalAttendees: 3, KeyStakeholder: 4}
	e := domain.CalendarEvent{Ref: domain.ExternalRef{Provider: "google_calendar", ID: "abc"}, StartAt: ctx.Now.Add(time.Hour)}
	score := Score(ctx, CalendarEventItem{Event: e})

	require.Len(t, score.Signals, 3)
	assert.Equal(t, "meeting_soon", score.Signals[0].Name)
	assert.Equal(t, "external_attendees", score.Signals[1].Name)
	assert.Equal(t, "key_stakeholder", score.Signals[2].Name)
}

func TestScore_ComputedAtMatchesNow(t *testing.T) {
	ctx := baseCtx()
	score := Score(ctx, IssueItem{Issue: domain.Issue{}})
	assert.Equal(t, ctx.Now, score.ComputedAt)
}

func TestScore_RefMatchesItem(t *testing.T) {
	ref := domain.ExternalRef{Provider: "linear", ID: "LIN-42"}
	score := Score(baseCtx(), IssueItem{Issue: domain.Issue{Ref: ref}})
	assert.Equal(t, ref, score.Ref)
}

func TestScore_SumsOnlyTriggeredWeights(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{UrgentIssue: 8, OverdueIssue: 5, StaleItem: 2}
	// Urgent fires (+8); not overdue (no DueAt); not stale (recent UpdatedAt).
	i := domain.Issue{Priority: domain.PriorityUrgent, UpdatedAt: ctx.Now}
	score := Score(ctx, IssueItem{Issue: i})
	assert.Equal(t, 8, score.Score)
}

func TestScore_DisabledSignalRecordedButContributesZero(t *testing.T) {
	ctx := baseCtx()
	// urgent_issue weight=0 means signal is disabled; it should still
	// appear in Signals so --explain can show "checked but disabled."
	ctx.Weights = Weights{UrgentIssue: 0}
	i := domain.Issue{Priority: domain.PriorityUrgent}
	score := Score(ctx, IssueItem{Issue: i})

	require.Len(t, score.Signals, 3)
	assert.True(t, score.Signals[0].Triggered)
	assert.Equal(t, 0, score.Signals[0].Weight)
	assert.Equal(t, 0, score.Score)
}

func TestScoreAll_SortedDescending(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{UrgentIssue: 10}

	low := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "low"}, Priority: domain.PriorityLow}}
	high := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "high"}, Priority: domain.PriorityUrgent}}
	mid := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "mid"}, Priority: domain.PriorityHigh}}

	out := ScoreAll(ctx, []Item{low, high, mid})
	require.Len(t, out, 3)
	// high (10) and mid (10) tie; low (0) last.
	assert.Equal(t, "high", out[0].Ref.ID)
	assert.Equal(t, "mid", out[1].Ref.ID) // input order for ties
	assert.Equal(t, "low", out[2].Ref.ID)
}

func TestScoreAll_StableOrderForTies(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{UrgentIssue: 10}

	a := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "a"}, Priority: domain.PriorityUrgent}}
	b := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "b"}, Priority: domain.PriorityUrgent}}
	c := IssueItem{Issue: domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: "c"}, Priority: domain.PriorityUrgent}}

	out := ScoreAll(ctx, []Item{a, b, c})
	require.Len(t, out, 3)
	assert.Equal(t, "a", out[0].Ref.ID)
	assert.Equal(t, "b", out[1].Ref.ID)
	assert.Equal(t, "c", out[2].Ref.ID)
}

func TestScoreAll_EmptySliceReturnsEmpty(t *testing.T) {
	out := ScoreAll(baseCtx(), nil)
	assert.NotNil(t, out, "ScoreAll returns an empty (non-nil) slice for an empty input")
	assert.Empty(t, out)
}

func TestDefaultThresholds(t *testing.T) {
	d := DefaultThresholds()
	assert.Equal(t, 30*time.Minute, d.MeetingSoonWindow)
	assert.Equal(t, 14*24*time.Hour, d.StaleAfter)
}

func TestThresholds_ZeroValueUsesDefaults(t *testing.T) {
	ctx := baseCtx()
	ctx.Weights = Weights{MeetingSoon: 5}
	ctx.Thresholds = Thresholds{} // zero — should resolve to default 30m window

	e := domain.CalendarEvent{StartAt: ctx.Now.Add(20 * time.Minute)}
	score := Score(ctx, CalendarEventItem{Event: e})
	require.NotEmpty(t, score.Signals)
	assert.True(t, score.Signals[0].Triggered, "meeting_soon should fire for an event 20 minutes out under default 30m window")
}
