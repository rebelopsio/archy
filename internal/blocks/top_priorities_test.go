package blocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

func TestTopPrioritiesBlock_Available_NoLinearSource(t *testing.T) {
	gctx := GatherContext{
		Sources: map[string]struct{}{"github": {}},
		Issues:  []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X-1"}}},
	}
	assert.False(t, TopPrioritiesBlock{}.Available(context.Background(), gctx))
}

func TestTopPrioritiesBlock_Available_NoIssues(t *testing.T) {
	gctx := GatherContext{
		Sources: map[string]struct{}{"linear": {}},
		Issues:  nil,
	}
	assert.False(t, TopPrioritiesBlock{}.Available(context.Background(), gctx))
}

func TestTopPrioritiesBlock_Available_True(t *testing.T) {
	gctx := GatherContext{
		Sources: map[string]struct{}{"linear": {}},
		Issues:  []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X-1"}}},
	}
	assert.True(t, TopPrioritiesBlock{}.Available(context.Background(), gctx))
}

func TestTopPrioritiesBlock_Gather_RanksAndLimits(t *testing.T) {
	issues := []domain.Issue{
		{Ref: domain.ExternalRef{Provider: "linear", ID: "A"}, Title: "alpha"},
		{Ref: domain.ExternalRef{Provider: "linear", ID: "B"}, Title: "bravo"},
		{Ref: domain.ExternalRef{Provider: "linear", ID: "C"}, Title: "charlie"},
		{Ref: domain.ExternalRef{Provider: "linear", ID: "D"}, Title: "delta"},
	}
	gctx := GatherContext{
		Sources: map[string]struct{}{"linear": {}},
		Issues:  issues,
		Scorer:  fakeScorer{order: []string{"C", "A", "D", "B"}}, // C highest
	}
	data, err := TopPrioritiesBlock{Limit: 2}.Gather(context.Background(), gctx)
	require.NoError(t, err)
	d, ok := data.(topPrioritiesData)
	require.True(t, ok)
	require.Len(t, d.Items, 2)
	assert.Equal(t, "C", d.Items[0].Issue.Ref.ID)
	assert.Equal(t, "A", d.Items[1].Issue.Ref.ID)
}

func TestTopPrioritiesBlock_Gather_DefaultLimitFive(t *testing.T) {
	issues := make([]domain.Issue, 7)
	order := make([]string, 7)
	for i := range issues {
		id := string(rune('A' + i))
		issues[i] = domain.Issue{Ref: domain.ExternalRef{Provider: "linear", ID: id}}
		order[i] = id
	}
	gctx := GatherContext{
		Sources: map[string]struct{}{"linear": {}},
		Issues:  issues,
		Scorer:  fakeScorer{order: order},
	}
	data, err := TopPrioritiesBlock{Limit: 0}.Gather(context.Background(), gctx)
	require.NoError(t, err)
	assert.Len(t, data.(topPrioritiesData).Items, 5)
}

func TestTopPrioritiesBlock_Render_WithSignals(t *testing.T) {
	data := topPrioritiesData{Items: []priorityItem{
		{
			Issue: domain.Issue{
				Ref:   domain.ExternalRef{Provider: "linear", ID: "ENG-1"},
				Title: "Fix the thing",
			},
			Score: domain.PriorityScore{
				Score: 13,
				Signals: []domain.ScoreSignal{
					{Name: "urgent_issue", Triggered: true, Reason: "priority: urgent"},
					{Name: "overdue_issue", Triggered: true, Reason: "overdue by 2 days"},
					{Name: "stale_item", Triggered: false, Reason: "updated 1 days ago"},
				},
			},
		},
	}}
	got, err := TopPrioritiesBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	want := "## Top Priorities\n\n- [ENG-1] Fix the thing (priority: urgent, overdue by 2 days)"
	assert.Equal(t, want, got)
}

func TestTopPrioritiesBlock_Render_OmitsParenthetical_WhenNoTriggered(t *testing.T) {
	data := topPrioritiesData{Items: []priorityItem{
		{
			Issue: domain.Issue{Ref: domain.ExternalRef{ID: "ENG-1"}, Title: "Untouched"},
			Score: domain.PriorityScore{
				Score: 0,
				Signals: []domain.ScoreSignal{
					{Name: "urgent_issue", Triggered: false, Reason: "priority: low"},
				},
			},
		},
	}}
	got, err := TopPrioritiesBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, "## Top Priorities\n\n- [ENG-1] Untouched", got)
}

func TestTopPrioritiesBlock_Render_MissingURL_StillUsesIDBracket(t *testing.T) {
	// URL is not part of the rendered output today (the PRD says ID
	// in brackets is what appears); regression-guard the behavior.
	data := topPrioritiesData{Items: []priorityItem{
		{
			Issue: domain.Issue{Ref: domain.ExternalRef{ID: "X-9"}, Title: "no url here"},
		},
	}}
	got, err := TopPrioritiesBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Contains(t, got, "[X-9]")
}

func TestTopPrioritiesBlock_Render_WrongDataTypeErrors(t *testing.T) {
	_, err := TopPrioritiesBlock{}.Render(context.Background(), 42)
	require.Error(t, err)
}
