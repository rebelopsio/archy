package blocks

import (
	"context"
	"fmt"
	"strings"

	"github.com/rebelopsio/archy/internal/domain"
)

// defaultTopPrioritiesLimit caps the rendered list when Limit is zero.
const defaultTopPrioritiesLimit = 5

// TopPrioritiesBlock ranks the user's open issues by computed priority
// and renders the top N as a bulleted list. The block's Available
// method gates on the "linear" source plus a non-empty issue list.
type TopPrioritiesBlock struct {
	// Limit caps the number of items rendered. Zero or negative
	// values fall back to defaultTopPrioritiesLimit (5).
	Limit int
}

// Name returns the stable block identifier "top_priorities".
func (TopPrioritiesBlock) Name() string { return "top_priorities" }

// Available returns true iff the run has Linear as a source AND there
// is at least one issue to score. An empty issue list is treated as
// "data not yet available"; the block is skipped rather than rendered
// as an empty section.
func (TopPrioritiesBlock) Available(_ context.Context, gctx GatherContext) bool {
	if _, ok := gctx.Sources["linear"]; !ok {
		return false
	}
	return len(gctx.Issues) > 0
}

// Gather scores the issues from gctx via gctx.Scorer, takes the top
// Limit, and returns the result for Render. Issues whose scores fail
// to match an issue in gctx (shouldn't happen in practice) are skipped.
func (b TopPrioritiesBlock) Gather(ctx context.Context, gctx GatherContext) (BlockData, error) {
	limit := b.Limit
	if limit <= 0 {
		limit = defaultTopPrioritiesLimit
	}
	scores := gctx.Scorer.ScoreIssues(ctx, gctx.Issues)
	byRef := make(map[domain.ExternalRef]domain.Issue, len(gctx.Issues))
	for _, i := range gctx.Issues {
		byRef[i.Ref] = i
	}
	items := make([]priorityItem, 0, limit)
	for _, s := range scores {
		if len(items) >= limit {
			break
		}
		if iss, ok := byRef[s.Ref]; ok {
			items = append(items, priorityItem{Issue: iss, Score: s})
		}
	}
	return topPrioritiesData{Items: items}, nil
}

// Render produces the bulleted list. Each item is "[<id>] <title>"
// followed by a parenthetical of triggered signal Reasons, joined by
// ", ". When a score has no triggered signals (Score == 0), the
// parenthetical is omitted.
func (TopPrioritiesBlock) Render(_ context.Context, data BlockData) (string, error) {
	d, ok := data.(topPrioritiesData)
	if !ok {
		return "", fmt.Errorf("top_priorities: render expected topPrioritiesData, got %T", data)
	}
	var sb strings.Builder
	sb.WriteString("## Top Priorities\n\n")
	for _, it := range d.Items {
		sb.WriteString("- [")
		sb.WriteString(it.Issue.Ref.ID)
		sb.WriteString("] ")
		sb.WriteString(it.Issue.Title)
		reasons := triggeredReasons(it.Score)
		if len(reasons) > 0 {
			sb.WriteString(" (")
			sb.WriteString(strings.Join(reasons, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// priorityItem pairs a domain.Issue with its computed score. Used as
// both top_priorities's Gather output (via [topPrioritiesData]) and
// synthesis's input via [GatherContext.PriorBlockData].
type priorityItem struct {
	Issue domain.Issue
	Score domain.PriorityScore
}

// topPrioritiesData is what TopPrioritiesBlock.Gather returns.
// Synthesis reads this through PriorBlockData["top_priorities"].
type topPrioritiesData struct {
	Items []priorityItem
}

// triggeredReasons returns the Reason strings from triggered signals,
// in the engine's stable order. Used both by top_priorities's
// parenthetical and as raw input to the synthesis block.
func triggeredReasons(s domain.PriorityScore) []string {
	out := make([]string, 0, len(s.Signals))
	for _, sig := range s.Signals {
		if sig.Triggered && sig.Reason != "" {
			out = append(out, sig.Reason)
		}
	}
	return out
}
