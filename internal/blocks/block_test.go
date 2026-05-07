package blocks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

// fakeBlock is a stand-in that lets registry tests verify behavior
// without depending on the concrete blocks.
type fakeBlock struct{ name string }

func (f fakeBlock) Name() string                                           { return f.name }
func (fakeBlock) Available(context.Context, GatherContext) bool            { return true }
func (fakeBlock) Gather(context.Context, GatherContext) (BlockData, error) { return nil, nil }
func (fakeBlock) Render(context.Context, BlockData) (string, error)        { return "", nil }

func TestRegistry_Register_Adds(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(fakeBlock{name: "alpha"}))

	got, ok := r.Get("alpha")
	require.True(t, ok)
	assert.Equal(t, "alpha", got.Name())
}

func TestRegistry_Register_DuplicateErrors(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(fakeBlock{name: "alpha"}))

	err := r.Register(fakeBlock{name: "alpha"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicateName))
	assert.Contains(t, err.Error(), "alpha")
}

func TestRegistry_Get_MissingReturnsFalse(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("missing")
	assert.False(t, ok)
}

func TestRegistry_Names_Sorted(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(fakeBlock{name: "charlie"}))
	require.NoError(t, r.Register(fakeBlock{name: "alpha"}))
	require.NoError(t, r.Register(fakeBlock{name: "bravo"}))

	got := r.Names()
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, got)
}

func TestRegistry_Names_EmptyRegistry(t *testing.T) {
	r := NewRegistry()
	assert.Empty(t, r.Names())
}

// fakeScorer assigns a fixed score to each issue based on its ID's
// position in a configured order.
type fakeScorer struct{ order []string } // ordered list of IDs, highest first

func (f fakeScorer) ScoreIssues(_ context.Context, issues []domain.Issue) []domain.PriorityScore {
	idx := make(map[string]int, len(f.order))
	for i, id := range f.order {
		idx[id] = len(f.order) - i
	}
	out := make([]domain.PriorityScore, 0, len(issues))
	for _, iss := range issues {
		score := idx[iss.Ref.ID] // 0 for unranked
		out = append(out, domain.PriorityScore{
			Ref:   iss.Ref,
			Score: score,
		})
	}
	// Sort descending by Score.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[i].Score {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
