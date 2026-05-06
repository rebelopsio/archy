package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	claude "github.com/partio-io/claude-agent-sdk-go"
)

// RunRequest describes a single skill execution.
type RunRequest struct {
	// SkillName identifies the skill to invoke. Must match a SKILL.md's
	// frontmatter name. Required.
	SkillName string

	// Prompt is the user-supplied or workflow-supplied prompt that
	// initiates the run. Required.
	Prompt string

	// ExtraSystemPrompt is appended to the SDK's system prompt for this
	// run. Use sparingly; most behavior should come from the skill itself.
	ExtraSystemPrompt string

	// ProgressFn, if non-nil, is called with progress events as the run
	// executes. Synchronous: the callback runs on the goroutine reading
	// the SDK's message stream — keep it fast and non-blocking.
	ProgressFn func(ProgressEvent)
}

// RunResult is the outcome of a successful run.
type RunResult struct {
	// Text is the final assistant text from the agent, concatenated
	// across all assistant turns. May be empty if the agent only
	// performed tool calls.
	Text string
	// ToolCalls lists every tool the agent invoked, in order.
	ToolCalls []ToolCallRecord
	// Turns is the count of model turns the agent took.
	Turns int
	// Duration is wall-clock time from Run entry to Run return.
	Duration time.Duration
	// CostUSD is the model's reported cost for this run, if available.
	// Zero means unknown.
	CostUSD float64
}

// ToolCallRecord is one tool invocation observed during a run.
type ToolCallRecord struct {
	// Name is the tool's MCP-prefixed name (e.g.,
	// "mcp__archy__archy_write_vault_note").
	Name string
	// Arguments is the JSON-decoded args the agent supplied.
	Arguments map[string]any
	// Result is the tool's text result, or empty if Error is set.
	Result string
	// Error is the tool error message, or empty on success.
	Error string
	// At is when the tool was invoked.
	At time.Time
}

// textPreviewLen is the byte length the runtime truncates assistant
// text to when emitting [ProgressTextChunk] events. The full text is
// still concatenated into [RunResult.Text].
const textPreviewLen = 200

// Run executes a skill and returns its result. Cancellation via ctx
// terminates the underlying SDK session and propagates ctx.Err() back
// (wrapped) to the caller. Partial results are not returned on
// cancellation.
func (r *Runtime) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.SkillName == "" {
		return nil, fmt.Errorf("%w: SkillName is required", ErrRun)
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("%w: Prompt is required", ErrRun)
	}

	opts, err := buildOptions(r.cfg, r.opts)
	if err != nil {
		return nil, err
	}
	opts = append(opts, claude.WithAppendSystemPrompt(systemPromptAddition(req)))

	emit := func(ev ProgressEvent) {
		if req.ProgressFn != nil {
			req.ProgressFn(ev)
		}
	}

	start := time.Now()
	emit(ProgressEvent{Kind: ProgressStart, At: start})

	res := &RunResult{}
	pending := make(map[string]*ToolCallRecord) // tool_use_id → in-flight record
	var assistantText strings.Builder
	systemSeen := false

	for msg, err := range r.runner.run(ctx, req.Prompt, opts) {
		if err != nil {
			emit(ProgressEvent{Kind: ProgressEnd, Message: "error: " + err.Error(), At: time.Now()})
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil, fmt.Errorf("agent run canceled: %w", ctx.Err())
			}
			return nil, fmt.Errorf("%w: %v", ErrRun, err)
		}
		if ctx.Err() != nil {
			emit(ProgressEvent{Kind: ProgressEnd, Message: "error: " + ctx.Err().Error(), At: time.Now()})
			return nil, fmt.Errorf("agent run canceled: %w", ctx.Err())
		}

		switch m := msg.(type) {
		case *claude.SystemMessage:
			if !systemSeen {
				systemSeen = true
			}
		case *claude.AssistantMessage:
			if m.Message != nil {
				processAssistant(m, emit, &assistantText, pending, res)
			}
		case *claude.UserMessage:
			if m.Message != nil {
				processUserToolResults(m, pending, res)
			}
		case *claude.ResultMessage:
			res.Turns = m.NumTurns
			if m.TotalCostUSD != nil {
				res.CostUSD = *m.TotalCostUSD
			}
			emit(ProgressEvent{
				Kind:    ProgressTurnComplete,
				Message: fmt.Sprintf("turn complete (%d turns, %dms)", m.NumTurns, m.DurationMs),
				At:      time.Now(),
			})
			if m.IsError {
				endMsg := "error: result"
				if m.Result != nil {
					endMsg = "error: " + *m.Result
				}
				emit(ProgressEvent{Kind: ProgressEnd, Message: endMsg, At: time.Now()})
				res.Text = assistantText.String()
				res.Duration = time.Since(start)
				return res, fmt.Errorf("%w: %s", ErrRun, endMsg)
			}
		}
	}

	res.Text = assistantText.String()
	res.Duration = time.Since(start)
	emit(ProgressEvent{Kind: ProgressEnd, Message: "completed", At: time.Now()})
	return res, nil
}

// systemPromptAddition is the one-line skill-invocation instruction the
// runtime appends via [claude.WithAppendSystemPrompt]. Skill authors
// can rely on the agent seeing this exact phrasing.
func systemPromptAddition(req RunRequest) string {
	addition := fmt.Sprintf("Use the %q skill for this task.", req.SkillName)
	if req.ExtraSystemPrompt != "" {
		addition += "\n\n" + req.ExtraSystemPrompt
	}
	return addition
}

// processAssistant translates one AssistantMessage into progress events
// and starts in-flight ToolCallRecords for any tool_use blocks.
func processAssistant(
	m *claude.AssistantMessage,
	emit func(ProgressEvent),
	assistantText *strings.Builder,
	pending map[string]*ToolCallRecord,
	res *RunResult,
) {
	for _, block := range m.Message.Content {
		switch b := block.(type) {
		case *claude.TextBlock:
			assistantText.WriteString(b.Text)
			emit(ProgressEvent{
				Kind:    ProgressTextChunk,
				Message: truncate(b.Text, textPreviewLen),
				At:      time.Now(),
			})
		case *claude.ToolUseBlock:
			rec := &ToolCallRecord{
				Name:      b.Name,
				Arguments: b.Input,
				At:        time.Now(),
			}
			pending[b.ID] = rec
			res.ToolCalls = append(res.ToolCalls, *rec)
			emit(ProgressEvent{
				Kind:     ProgressToolCall,
				Message:  "calling " + b.Name,
				ToolName: b.Name,
				At:       rec.At,
			})
		}
	}
}

// processUserToolResults finalizes ToolCallRecords whose tool_use_id
// matches a tool_result block in this UserMessage. The record was
// appended to [RunResult.ToolCalls] when the tool_use was first seen;
// finalizing here updates Result/Error in place via index lookup.
func processUserToolResults(
	m *claude.UserMessage,
	pending map[string]*ToolCallRecord,
	res *RunResult,
) {
	for _, block := range m.Message.Content {
		tr, ok := block.(*claude.ToolResultBlock)
		if !ok {
			continue
		}
		_, found := pending[tr.ToolUseID]
		if !found {
			continue
		}
		text := toolResultText(tr.Content)
		// Find the matching record in res.ToolCalls by Name+At
		// (At is unique because pending entries are added per
		// observation). Prefer the latest matching record.
		for i := len(res.ToolCalls) - 1; i >= 0; i-- {
			if res.ToolCalls[i].Name == pending[tr.ToolUseID].Name &&
				res.ToolCalls[i].At.Equal(pending[tr.ToolUseID].At) {
				if tr.IsError {
					res.ToolCalls[i].Error = text
				} else {
					res.ToolCalls[i].Result = text
				}
				break
			}
		}
		delete(pending, tr.ToolUseID)
	}
}

// toolResultText flattens a tool-result Content (string or
// []ContentBlock) into a single string for the record.
func toolResultText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []claude.ContentBlock:
		var sb strings.Builder
		for _, blk := range c {
			if t, ok := blk.(*claude.TextBlock); ok {
				sb.WriteString(t.Text)
			}
		}
		return sb.String()
	default:
		return fmt.Sprintf("%v", content)
	}
}

// truncate returns s clipped to at most n bytes, suffixed with "..."
// if it was clipped. Byte-precision is fine for progress previews;
// rune-boundary preservation is not worth the complexity here.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
