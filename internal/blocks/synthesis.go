package blocks

import (
	"context"
	"fmt"
	"strings"
)

// SynthesisBlock produces a short narrative summary of the workflow's
// gathered data.
//
// TODO(real-llm-synthesis): Replace deterministic phrasing with the
// LLM-driven Synthesizer bridge from ADR-0004 once that PRD lands. The
// bridge keeps this block in internal/blocks; only the Render
// implementation changes when the AgentSynthesizer is wired in.
type SynthesisBlock struct {
	// Style is the requested tone/length. v1's deterministic
	// implementation ignores this; reserved so the field is part of
	// the contract when real synthesis arrives.
	Style string
}

// Name returns the stable block identifier "synthesis".
func (SynthesisBlock) Name() string { return "synthesis" }

// Available returns true unconditionally — synthesis can summarize
// over whatever data the prior blocks gathered, including an empty
// set (the empty case has its own canned phrasing).
func (SynthesisBlock) Available(_ context.Context, _ GatherContext) bool {
	return true
}

// Gather reads PriorBlockData["top_priorities"] and packages its
// items for Render. If top_priorities was skipped (no Linear source,
// no issues), Gather returns an empty data set so Render produces the
// "nothing pressing" output.
func (SynthesisBlock) Gather(_ context.Context, gctx GatherContext) (BlockData, error) {
	prior, ok := gctx.PriorBlockData["top_priorities"]
	if !ok {
		return synthesisData{}, nil
	}
	td, ok := prior.(topPrioritiesData)
	if !ok {
		return nil, fmt.Errorf("synthesis: prior top_priorities data has unexpected type %T", prior)
	}
	return synthesisData(td), nil
}

// Render produces deterministic markdown driven by signal patterns
// on the top items. The exact phrasing is documented in the
// daily-brief PRD; this implementation is the test contract.
func (SynthesisBlock) Render(_ context.Context, data BlockData) (string, error) {
	d, ok := data.(synthesisData)
	if !ok {
		return "", fmt.Errorf("synthesis: render expected synthesisData, got %T", data)
	}
	var sb strings.Builder
	sb.WriteString("## Suggested Plan\n\n")
	if len(d.Items) == 0 {
		sb.WriteString("Nothing pressing today.")
		return sb.String(), nil
	}

	first := d.Items[0]
	urgent := signalTriggered(first, "urgent_issue")
	overdue := signalTriggered(first, "overdue_issue")
	switch {
	case urgent && overdue:
		fmt.Fprintf(&sb, "Focus on %s first — it's urgent and overdue.", first.Issue.Ref.ID)
	case urgent:
		fmt.Fprintf(&sb, "Focus on %s first — it's urgent.", first.Issue.Ref.ID)
	case overdue:
		fmt.Fprintf(&sb, "Focus on %s first — it's overdue.", first.Issue.Ref.ID)
	default:
		fmt.Fprintf(&sb, "Focus on %s first.", first.Issue.Ref.ID)
	}

	if len(d.Items) >= 2 {
		fmt.Fprintf(&sb, " After that, review %s.", d.Items[1].Issue.Ref.ID)
	}
	if len(d.Items) >= 3 {
		sb.WriteString(" Then triage the rest.")
	}
	return sb.String(), nil
}

// synthesisData is what SynthesisBlock.Gather returns. Reuses
// [priorityItem] from top_priorities so the synthesis block can read
// both the issue and its scoring signals without duplicating types.
type synthesisData struct {
	Items []priorityItem
}

// signalTriggered reports whether item's score triggered a signal
// with the given name.
func signalTriggered(item priorityItem, name string) bool {
	for _, s := range item.Score.Signals {
		if s.Name == name {
			return s.Triggered
		}
	}
	return false
}
