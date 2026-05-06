package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityScore_ZeroValue(t *testing.T) {
	var s PriorityScore
	assert.Equal(t, 0, s.Score)
	assert.Empty(t, s.Signals)
	assert.True(t, s.Ref.IsZero())
}

func TestScoreSignal_NotTriggered(t *testing.T) {
	// A signal that was checked but did not fire is a valid representable
	// state — the scoring engine records false signals so --explain can
	// show what was checked.
	sig := ScoreSignal{Name: "meeting_soon", Weight: 5, Triggered: false, Reason: "no meetings in next 60min"}
	assert.Equal(t, "meeting_soon", sig.Name)
	assert.Equal(t, 5, sig.Weight)
	assert.False(t, sig.Triggered)
	assert.Equal(t, "no meetings in next 60min", sig.Reason)
}
