package blocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

// pi builds a priorityItem with a small set of triggered signals.
// Names are the engine's stable identifiers; only the triggered flag
// matters to synthesis.
func pi(id string, triggered ...string) priorityItem {
	signals := make([]domain.ScoreSignal, 0, len(triggered))
	for _, n := range triggered {
		signals = append(signals, domain.ScoreSignal{Name: n, Triggered: true})
	}
	return priorityItem{
		Issue: domain.Issue{Ref: domain.ExternalRef{ID: id}},
		Score: domain.PriorityScore{Signals: signals},
	}
}

func TestSynthesisBlock_Available_AlwaysTrue(t *testing.T) {
	assert.True(t, SynthesisBlock{}.Available(context.Background(), GatherContext{}))
}

func TestSynthesisBlock_Render_NoItems(t *testing.T) {
	got, err := SynthesisBlock{}.Render(context.Background(), synthesisData{})
	require.NoError(t, err)
	assert.Equal(t, "## Suggested Plan\n\nNothing pressing today.", got)
}

func TestSynthesisBlock_Render_OneItemUrgentAndOverdue(t *testing.T) {
	data := synthesisData{Items: []priorityItem{
		pi("ENG-1", "urgent_issue", "overdue_issue"),
	}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, "## Suggested Plan\n\nFocus on ENG-1 first — it's urgent and overdue.", got)
}

func TestSynthesisBlock_Render_OneItemUrgentOnly(t *testing.T) {
	data := synthesisData{Items: []priorityItem{pi("ENG-1", "urgent_issue")}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, "## Suggested Plan\n\nFocus on ENG-1 first — it's urgent.", got)
}

func TestSynthesisBlock_Render_OneItemOverdueOnly(t *testing.T) {
	data := synthesisData{Items: []priorityItem{pi("ENG-1", "overdue_issue")}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, "## Suggested Plan\n\nFocus on ENG-1 first — it's overdue.", got)
}

func TestSynthesisBlock_Render_OneItemNeitherFires(t *testing.T) {
	data := synthesisData{Items: []priorityItem{pi("ENG-1")}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, "## Suggested Plan\n\nFocus on ENG-1 first.", got)
}

func TestSynthesisBlock_Render_TwoItems(t *testing.T) {
	data := synthesisData{Items: []priorityItem{
		pi("ENG-1", "urgent_issue"),
		pi("SOC-2"),
	}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t,
		"## Suggested Plan\n\nFocus on ENG-1 first — it's urgent. After that, review SOC-2.",
		got)
}

func TestSynthesisBlock_Render_ThreeOrMoreItems(t *testing.T) {
	data := synthesisData{Items: []priorityItem{
		pi("ENG-1", "urgent_issue", "overdue_issue"),
		pi("SOC-2"),
		pi("OPS-3"),
		pi("OPS-4"),
	}}
	got, err := SynthesisBlock{}.Render(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t,
		"## Suggested Plan\n\nFocus on ENG-1 first — it's urgent and overdue. After that, review SOC-2. Then triage the rest.",
		got)
}

func TestSynthesisBlock_Gather_FromPriorBlockData(t *testing.T) {
	prior := topPrioritiesData{Items: []priorityItem{pi("ENG-1", "urgent_issue")}}
	gctx := GatherContext{PriorBlockData: map[string]BlockData{
		"top_priorities": prior,
	}}
	data, err := SynthesisBlock{}.Gather(context.Background(), gctx)
	require.NoError(t, err)
	d, ok := data.(synthesisData)
	require.True(t, ok)
	require.Len(t, d.Items, 1)
	assert.Equal(t, "ENG-1", d.Items[0].Issue.Ref.ID)
}

func TestSynthesisBlock_Gather_NoPriorReturnsEmpty(t *testing.T) {
	gctx := GatherContext{}
	data, err := SynthesisBlock{}.Gather(context.Background(), gctx)
	require.NoError(t, err)
	d, ok := data.(synthesisData)
	require.True(t, ok)
	assert.Empty(t, d.Items)
}

func TestSynthesisBlock_Gather_WrongPriorTypeErrors(t *testing.T) {
	gctx := GatherContext{PriorBlockData: map[string]BlockData{
		"top_priorities": "wrong type",
	}}
	_, err := SynthesisBlock{}.Gather(context.Background(), gctx)
	require.Error(t, err)
}

func TestSynthesisBlock_Render_WrongDataTypeErrors(t *testing.T) {
	_, err := SynthesisBlock{}.Render(context.Background(), "wrong")
	require.Error(t, err)
}
