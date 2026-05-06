package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleScoreItems_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{})
	require.NoError(t, err)
	assert.Empty(t, out.Scores)
}

func TestHandleScoreItems_UrgentIssue(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind: "issue",
			Issue: &IssuePayload{
				Ref:      RefPayload{Provider: "linear", ID: "LIN-1"},
				Priority: "urgent",
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 1)
	require.NotEmpty(t, out.Scores[0].Signals)
	// urgent_issue is the first signal in the issue order
	assert.Equal(t, "urgent_issue", out.Scores[0].Signals[0].Name)
	assert.True(t, out.Scores[0].Signals[0].Triggered)
	assert.Greater(t, out.Scores[0].Score, 0)
}

func TestHandleScoreItems_ReviewRequested(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind: "pull_request",
			PullRequest: &PRPayload{
				Ref:                RefPayload{Provider: "github", ID: "1"},
				State:              "open",
				RequestedReviewers: []PersonPayload{{Username: "user"}},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 1)
	// review_requested is the first PR signal
	assert.Equal(t, "review_requested", out.Scores[0].Signals[0].Name)
	assert.True(t, out.Scores[0].Signals[0].Triggered)
}

func TestHandleScoreItems_MeetingSoon(t *testing.T) {
	srv, _ := newTestServer(t)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	srv.setNow(func() time.Time { return now })

	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind: "calendar_event",
			CalendarEvent: &CalEventPayload{
				Ref:     RefPayload{Provider: "google_calendar", ID: "abc"},
				StartAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano),
				EndAt:   now.Add(40 * time.Minute).Format(time.RFC3339Nano),
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 1)
	assert.Equal(t, "meeting_soon", out.Scores[0].Signals[0].Name)
	assert.True(t, out.Scores[0].Signals[0].Triggered)
}

func TestHandleScoreItems_SortedDescending(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{
			{Kind: "issue", Issue: &IssuePayload{Ref: RefPayload{Provider: "linear", ID: "low"}, Priority: "low"}},
			{Kind: "issue", Issue: &IssuePayload{Ref: RefPayload{Provider: "linear", ID: "urgent"}, Priority: "urgent"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 2)
	assert.Equal(t, "urgent", out.Scores[0].Ref.ID)
	assert.Equal(t, "low", out.Scores[1].Ref.ID)
}

func TestHandleScoreItems_SignalsInStableOrder(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{Kind: "issue", Issue: &IssuePayload{Ref: RefPayload{Provider: "linear", ID: "x"}, Priority: "urgent"}}},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 1)
	require.Len(t, out.Scores[0].Signals, 3)
	assert.Equal(t, "urgent_issue", out.Scores[0].Signals[0].Name)
	assert.Equal(t, "overdue_issue", out.Scores[0].Signals[1].Name)
	assert.Equal(t, "stale_item", out.Scores[0].Signals[2].Name)
}

func TestHandleScoreItems_CIPassingFalse_TriggersCIFailing(t *testing.T) {
	srv, _ := newTestServer(t)
	fail := false
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind: "pull_request",
			PullRequest: &PRPayload{
				Ref: RefPayload{Provider: "github", ID: "1"}, State: "open", CIPassing: &fail,
			},
		}},
	})
	require.NoError(t, err)
	// ci_failing is the third PR signal (review_requested, blocked_on_user, ci_failing, stale_item)
	require.Len(t, out.Scores[0].Signals, 4)
	assert.Equal(t, "ci_failing", out.Scores[0].Signals[2].Name)
	assert.True(t, out.Scores[0].Signals[2].Triggered)
}

func TestHandleScoreItems_CIPassingAbsent_ReportsUnknown(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind:        "pull_request",
			PullRequest: &PRPayload{Ref: RefPayload{Provider: "github", ID: "1"}, State: "open"},
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, "ci_failing", out.Scores[0].Signals[2].Name)
	assert.False(t, out.Scores[0].Signals[2].Triggered)
	assert.Equal(t, "CI status unknown", out.Scores[0].Signals[2].Reason)
}

func TestHandleScoreItems_UnknownKind_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{Kind: "bogus"}},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

func TestHandleScoreItems_UsesInjectedClock(t *testing.T) {
	srv, _ := newTestServer(t)
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	srv.setNow(func() time.Time { return fixed })

	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{Kind: "issue", Issue: &IssuePayload{Ref: RefPayload{Provider: "linear", ID: "x"}, Priority: "urgent"}}},
	})
	require.NoError(t, err)
	require.Len(t, out.Scores, 1)

	parsed, err := time.Parse(time.RFC3339Nano, out.Scores[0].ComputedAt)
	require.NoError(t, err)
	assert.True(t, parsed.Equal(fixed), "ComputedAt should reflect the injected clock")
}

func TestHandleScoreItems_RFC3339Roundtrip(t *testing.T) {
	srv, _ := newTestServer(t)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	srv.setNow(func() time.Time { return now })

	due := now.Add(-3 * 24 * time.Hour)
	_, out, err := srv.handleScoreItems(context.Background(), &mcp.CallToolRequest{}, ScoreItemsInput{
		Items: []ScoreItem{{
			Kind: "issue",
			Issue: &IssuePayload{
				Ref:      RefPayload{Provider: "linear", ID: "x"},
				State:    "in_progress",
				Priority: "high",
				DueAt:    due.Format(time.RFC3339Nano),
			},
		}},
	})
	require.NoError(t, err)
	// overdue_issue should be triggered ("overdue by 3 days")
	for _, sig := range out.Scores[0].Signals {
		if sig.Name == "overdue_issue" {
			assert.True(t, sig.Triggered)
			assert.Equal(t, "overdue by 3 days", sig.Reason)
		}
	}
}
