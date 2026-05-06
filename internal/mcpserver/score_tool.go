package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/scoring"
)

// ScoreItemsInput is the typed input for archy_score_items.
type ScoreItemsInput struct {
	// Items to score, each tagged with kind.
	Items []ScoreItem `json:"items" jsonschema:"items to score, each tagged with kind"`
}

// ScoreItem is one item in a scoring request, polymorphic via Kind.
type ScoreItem struct {
	// Kind selects which payload field is consulted.
	Kind string `json:"kind" jsonschema:"one of: issue, pull_request, calendar_event"`
	// Issue is the payload when Kind is "issue".
	Issue *IssuePayload `json:"issue,omitempty"`
	// PullRequest is the payload when Kind is "pull_request".
	PullRequest *PRPayload `json:"pull_request,omitempty"`
	// CalendarEvent is the payload when Kind is "calendar_event".
	CalendarEvent *CalEventPayload `json:"calendar_event,omitempty"`
}

// IssuePayload mirrors [domain.Issue] for the scoring tool's wire format.
// Only fields scoring signals consult are included.
type IssuePayload struct {
	Ref       RefPayload `json:"ref"`
	Title     string     `json:"title,omitempty"`
	State     string     `json:"state,omitempty"`
	Priority  string     `json:"priority,omitempty"`
	DueAt     string     `json:"due_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

// PRPayload mirrors [domain.PullRequest] for the scoring tool.
type PRPayload struct {
	Ref                RefPayload      `json:"ref"`
	Title              string          `json:"title,omitempty"`
	State              string          `json:"state,omitempty"`
	RequestedReviewers []PersonPayload `json:"requested_reviewers,omitempty"`
	CIPassing          *bool           `json:"ci_passing,omitempty"`
	UpdatedAt          string          `json:"updated_at,omitempty"`
}

// CalEventPayload mirrors [domain.CalendarEvent] for the scoring tool.
type CalEventPayload struct {
	Ref       RefPayload      `json:"ref"`
	Title     string          `json:"title,omitempty"`
	StartAt   string          `json:"start_at,omitempty"`
	EndAt     string          `json:"end_at,omitempty"`
	AllDay    bool            `json:"all_day,omitempty"`
	Attendees []PersonPayload `json:"attendees,omitempty"`
	Organizer *PersonPayload  `json:"organizer,omitempty"`
}

// RefPayload mirrors [domain.ExternalRef].
type RefPayload struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty"`
}

// PersonPayload mirrors [domain.Person] for the fields scoring needs.
type PersonPayload struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
}

// ScoreItemsOutput is the typed output for archy_score_items.
type ScoreItemsOutput struct {
	// Scores in descending order by Score; ties preserve input order.
	Scores []ScoredItem `json:"scores"`
}

// ScoredItem is one scored item in the response.
type ScoredItem struct {
	Ref        RefPayload      `json:"ref"`
	Score      int             `json:"score"`
	Signals    []SignalPayload `json:"signals"`
	ComputedAt string          `json:"computed_at"`
}

// SignalPayload is one signal entry in a [ScoredItem].
type SignalPayload struct {
	Name      string `json:"name"`
	Weight    int    `json:"weight"`
	Triggered bool   `json:"triggered"`
	Reason    string `json:"reason,omitempty"`
}

// handleScoreItems decodes payloads into domain types, runs them
// through [scoring.ScoreAll], and returns [ScoredItem]s. Unknown Kind
// values surface as a tool-level error.
func (s *Server) handleScoreItems(
	_ context.Context,
	_ *mcp.CallToolRequest,
	in ScoreItemsInput,
) (*mcp.CallToolResult, ScoreItemsOutput, error) {
	items := make([]scoring.Item, 0, len(in.Items))
	for i, raw := range in.Items {
		item, err := decodeScoreItem(raw)
		if err != nil {
			return toolError(fmt.Sprintf("items[%d]: %s", i, err.Error())), ScoreItemsOutput{}, nil
		}
		items = append(items, item)
	}

	now := s.now()
	ctx := scoring.Context{
		Now:             now,
		UserEmail:       s.cfg.UserEmail,
		UserUsername:    s.cfg.UserUsername,
		KeyStakeholders: s.cfg.KeyStakeholders,
		Weights:         s.cfg.ScoringWeights,
		Thresholds:      s.cfg.ScoringThresholds,
	}
	scored := scoring.ScoreAll(ctx, items)

	out := ScoreItemsOutput{Scores: make([]ScoredItem, 0, len(scored))}
	for _, ps := range scored {
		out.Scores = append(out.Scores, scoredItemFromPriorityScore(ps))
	}
	return nil, out, nil
}

func scoredItemFromPriorityScore(ps domain.PriorityScore) ScoredItem {
	signals := make([]SignalPayload, 0, len(ps.Signals))
	for _, sig := range ps.Signals {
		signals = append(signals, SignalPayload{
			Name:      sig.Name,
			Weight:    sig.Weight,
			Triggered: sig.Triggered,
			Reason:    sig.Reason,
		})
	}
	return ScoredItem{
		Ref:        RefPayload{Provider: ps.Ref.Provider, ID: ps.Ref.ID, URL: ps.Ref.URL},
		Score:      ps.Score,
		Signals:    signals,
		ComputedAt: ps.ComputedAt.UTC().Format(rfc3339),
	}
}
