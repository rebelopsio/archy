package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	claude "github.com/partio-io/claude-agent-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers -----------------------------------------------------------

func textBlock(s string) claude.ContentBlock { return &claude.TextBlock{Type: "text", Text: s} }

func toolUseBlock(id, name string, input map[string]any) claude.ContentBlock {
	return &claude.ToolUseBlock{Type: "tool_use", ID: id, Name: name, Input: input}
}

func toolResultBlock(toolUseID string, content any, isError bool) claude.ContentBlock {
	return &claude.ToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: content, IsError: isError}
}

func assistantMsg(blocks ...claude.ContentBlock) claude.Message {
	return &claude.AssistantMessage{
		Type:    "assistant",
		Message: &claude.APIMessage{Type: "message", Role: "assistant", Content: blocks, RawContent: rawContent(blocks)},
	}
}

func userToolResultsMsg(blocks ...claude.ContentBlock) claude.Message {
	return &claude.UserMessage{
		Type:    "user",
		Message: &claude.UserAPIMessage{Content: blocks},
	}
}

func systemMsg() claude.Message {
	return &claude.SystemMessage{Type: "system", Subtype: "init"}
}

func resultMsg(turns, durationMs int, isError bool, result *string) claude.Message {
	return &claude.ResultMessage{
		Type:       "result",
		Subtype:    claude.ResultSuccess,
		NumTurns:   turns,
		DurationMs: durationMs,
		IsError:    isError,
		Result:     result,
	}
}

func rawContent(blocks []claude.ContentBlock) json.RawMessage {
	// minimal raw bytes to satisfy any code that re-marshals; the
	// Content slice is what processAssistant actually consumes.
	b, _ := json.Marshal([]struct{}{})
	return b
}

func ptr[T any](v T) *T { return &v }

// tests -------------------------------------------------------------

func TestRun_TranslatesAssistantTextIntoRunResult(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			assistantMsg(textBlock("hello"), textBlock(", world")),
			resultMsg(1, 50, false, nil),
		},
	}
	rt := newTestRuntime(t, fr)

	res, err := rt.Run(context.Background(), RunRequest{
		SkillName: "daily-brief",
		Prompt:    "do a thing",
	})
	require.NoError(t, err)
	assert.Equal(t, "hello, world", res.Text)
	assert.Equal(t, 1, res.Turns)
	assert.Greater(t, res.Duration, time.Duration(0))
}

func TestRun_RecordsToolCallsInOrderAndPairsResults(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			assistantMsg(
				toolUseBlock("u1", "mcp__archy__archy_score_items", map[string]any{"items": []any{}}),
			),
			userToolResultsMsg(
				toolResultBlock("u1", "{\"scores\":[]}", false),
			),
			assistantMsg(
				toolUseBlock("u2", "mcp__archy__archy_write_vault_note", map[string]any{"path": "x"}),
			),
			userToolResultsMsg(
				toolResultBlock("u2", "ok", false),
			),
			resultMsg(2, 100, false, nil),
		},
	}
	rt := newTestRuntime(t, fr)

	res, err := rt.Run(context.Background(), RunRequest{SkillName: "daily-brief", Prompt: "go"})
	require.NoError(t, err)
	require.Len(t, res.ToolCalls, 2)
	assert.Equal(t, "mcp__archy__archy_score_items", res.ToolCalls[0].Name)
	assert.Equal(t, "{\"scores\":[]}", res.ToolCalls[0].Result)
	assert.Empty(t, res.ToolCalls[0].Error)
	assert.Equal(t, "mcp__archy__archy_write_vault_note", res.ToolCalls[1].Name)
	assert.Equal(t, "ok", res.ToolCalls[1].Result)
}

func TestRun_RecordsToolErrors(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			assistantMsg(toolUseBlock("u1", "mcp__archy__archy_write_vault_note", nil)),
			userToolResultsMsg(toolResultBlock("u1", "path escapes vault root", true)),
			resultMsg(1, 30, false, nil),
		},
	}
	rt := newTestRuntime(t, fr)
	res, err := rt.Run(context.Background(), RunRequest{SkillName: "daily-brief", Prompt: "go"})
	require.NoError(t, err)
	require.Len(t, res.ToolCalls, 1)
	assert.Equal(t, "path escapes vault root", res.ToolCalls[0].Error)
	assert.Empty(t, res.ToolCalls[0].Result)
}

func TestRun_TurnCountFromResultMessage(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			resultMsg(7, 250, false, nil),
		},
	}
	rt := newTestRuntime(t, fr)
	res, err := rt.Run(context.Background(), RunRequest{SkillName: "x", Prompt: "go"})
	require.NoError(t, err)
	assert.Equal(t, 7, res.Turns)
}

func TestRun_CostUSDFromResultMessage(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			resultMsg(1, 50, false, nil),
		},
	}
	// Replace last message with one carrying TotalCostUSD set.
	fr.messages[1] = &claude.ResultMessage{Type: "result", NumTurns: 1, DurationMs: 50, TotalCostUSD: ptr(0.0123)}

	rt := newTestRuntime(t, fr)
	res, err := rt.Run(context.Background(), RunRequest{SkillName: "x", Prompt: "go"})
	require.NoError(t, err)
	assert.InDelta(t, 0.0123, res.CostUSD, 1e-6)
}

func TestRun_ProgressEventsInOrder(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			assistantMsg(textBlock("greetings")),
			assistantMsg(toolUseBlock("u1", "mcp__archy__archy_write_vault_note", nil)),
			userToolResultsMsg(toolResultBlock("u1", "ok", false)),
			resultMsg(1, 30, false, nil),
		},
	}
	rt := newTestRuntime(t, fr)

	var kinds []ProgressKind
	_, err := rt.Run(context.Background(), RunRequest{
		SkillName:  "daily-brief",
		Prompt:     "go",
		ProgressFn: func(ev ProgressEvent) { kinds = append(kinds, ev.Kind) },
	})
	require.NoError(t, err)
	require.Contains(t, kinds, ProgressStart)
	require.Contains(t, kinds, ProgressTextChunk)
	require.Contains(t, kinds, ProgressToolCall)
	require.Contains(t, kinds, ProgressTurnComplete)
	require.Contains(t, kinds, ProgressEnd)

	// Order: Start ... End at the boundaries.
	assert.Equal(t, ProgressStart, kinds[0])
	assert.Equal(t, ProgressEnd, kinds[len(kinds)-1])
}

func TestRun_SDKErrorReturnsErrRun(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{systemMsg()},
		terminal: errors.New("connection broken"),
	}
	rt := newTestRuntime(t, fr)
	_, err := rt.Run(context.Background(), RunRequest{SkillName: "x", Prompt: "go"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRun))
	assert.Contains(t, err.Error(), "connection broken")
}

func TestRun_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			assistantMsg(textBlock("first")),
			assistantMsg(textBlock("second")),
			resultMsg(1, 50, false, nil),
		},
		beforeYield: func(ctx context.Context) {
			// Cancel after we've emitted at least the system message.
			cancel()
		},
	}
	rt := newTestRuntime(t, fr)
	_, err := rt.Run(ctx, RunRequest{SkillName: "x", Prompt: "go"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRun_AppendsSkillSystemPromptInstruction(t *testing.T) {
	// Verify systemPromptAddition is constructed correctly.
	got := systemPromptAddition(RunRequest{SkillName: "daily-brief"})
	assert.Equal(t, `Use the "daily-brief" skill for this task.`, got)

	gotExtra := systemPromptAddition(RunRequest{SkillName: "daily-brief", ExtraSystemPrompt: "Be terse."})
	assert.True(t, strings.HasPrefix(gotExtra, `Use the "daily-brief" skill for this task.`))
	assert.Contains(t, gotExtra, "Be terse.")
}

func TestRun_SkillNameRequired(t *testing.T) {
	rt := newTestRuntime(t, &fakeRunner{messages: []claude.Message{systemMsg(), resultMsg(0, 0, false, nil)}})
	_, err := rt.Run(context.Background(), RunRequest{Prompt: "go"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRun))
}

func TestRun_PromptRequired(t *testing.T) {
	rt := newTestRuntime(t, &fakeRunner{messages: []claude.Message{systemMsg(), resultMsg(0, 0, false, nil)}})
	_, err := rt.Run(context.Background(), RunRequest{SkillName: "x"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRun))
}

func TestRun_DurationMeasuredWallClock(t *testing.T) {
	fr := &fakeRunner{
		messages: []claude.Message{
			systemMsg(),
			resultMsg(1, 50, false, nil),
		},
		beforeYield: func(_ context.Context) { time.Sleep(5 * time.Millisecond) },
	}
	rt := newTestRuntime(t, fr)
	res, err := rt.Run(context.Background(), RunRequest{SkillName: "x", Prompt: "go"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Duration, 5*time.Millisecond)
}

func TestRun_PassesPromptToSDK(t *testing.T) {
	fr := &fakeRunner{messages: []claude.Message{systemMsg(), resultMsg(1, 0, false, nil)}}
	rt := newTestRuntime(t, fr)
	_, err := rt.Run(context.Background(), RunRequest{SkillName: "x", Prompt: "the user prompt"})
	require.NoError(t, err)
	assert.Equal(t, "the user prompt", fr.prompt)
}
