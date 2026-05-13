/*
Copyright © 2026 Stephen Morgan <steve@rebelops.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/agent"
	"github.com/rebelopsio/archy/internal/blocks"
	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/render"
	"github.com/rebelopsio/archy/internal/state"
	"github.com/rebelopsio/archy/internal/voice"
)

// fakeIssueGatherer is a stand-in IssueGatherer for runDaily tests.
type fakeIssueGatherer struct {
	issues []domain.Issue
	err    error
	calls  int
}

func (f *fakeIssueGatherer) GatherIssues(_ context.Context) ([]domain.Issue, error) {
	f.calls++
	return f.issues, f.err
}

// fakeDailyRuntime records every Run call and returns scripted results.
// onRun, when set, fires after the request is recorded; tests use it
// to simulate the side effects a real MCP server would have produced
// (e.g., creating the file the post-run verification step expects).
type fakeDailyRuntime struct {
	calls   []agent.RunRequest
	result  *agent.RunResult
	err     error
	closed  bool
	prompts []string
	onRun   func(req agent.RunRequest)
}

func (f *fakeDailyRuntime) Run(_ context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	f.calls = append(f.calls, req)
	f.prompts = append(f.prompts, req.Prompt)
	if f.onRun != nil {
		f.onRun(req)
	}
	return f.result, f.err
}

func (f *fakeDailyRuntime) Close() error {
	f.closed = true
	return nil
}

// fixtureScorer mirrors the real scoring logic minimally: every issue
// gets a fixed score with one triggered signal so the synthesis branch
// has something to assert on.
type fixtureScorer struct{}

func (fixtureScorer) ScoreIssues(_ context.Context, issues []domain.Issue) []domain.PriorityScore {
	out := make([]domain.PriorityScore, 0, len(issues))
	for _, iss := range issues {
		signals := []domain.ScoreSignal{
			{Name: "urgent_issue", Weight: 8, Triggered: iss.Priority == domain.PriorityUrgent, Reason: "priority: " + iss.Priority.String()},
		}
		score := 0
		if iss.Priority == domain.PriorityUrgent {
			score = 8
		}
		out = append(out, domain.PriorityScore{Ref: iss.Ref, Score: score, Signals: signals})
	}
	return out
}

// fixtureDeps builds a runDaily dependency set with a fresh state
// store, a baseline config, and the daily-brief template + blocks
// registered. Tests override the issueGatherer and runtime.
func fixtureDeps(t *testing.T) (dailyDeps, *fakeIssueGatherer, *fakeDailyRuntime, *bytes.Buffer) {
	t.Helper()
	vault := t.TempDir()
	store, err := state.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfg := &config.Config{
		Vault: config.VaultConfig{
			Path:    vault,
			Folders: config.VaultFolders{Daily: "Daily", Meetings: "Meetings", Triage: "Triage", Reviews: "Reviews", Inbox: "Inbox"},
		},
		MCPServers: map[string]config.MCPServerConfig{
			"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true, BearerTokenEnv: "ARCHY_LINEAR_TOKEN"},
		},
		Skills: config.SkillsConfig{ProjectDir: ".claude/skills", UserDir: "/home/user/.claude/skills"},
		Agent:  config.AgentConfig{Model: "claude-sonnet-4-5", MaxTurns: 30, PermissionMode: "acceptEdits"},
		State:  config.StateConfig{SQLitePath: "/tmp/state.db", CacheTTL: 15 * time.Minute},
		Output: config.OutputConfig{DefaultWriteMode: "marker-block", Timezone: "Local", Voice: true, Signature: false},
	}

	tpl := &render.Template{
		Name:        "daily-brief",
		WriteMode:   "marker-block",
		MarkerID:    "daily-brief",
		Frontmatter: map[string]any{"type": "daily-brief"},
		Blocks: []render.BlockSpec{
			{Name: "top_priorities", Config: map[string]any{"limit": 5}},
			{Name: "synthesis", Config: map[string]any{"style": "brief"}},
		},
	}
	reg, err := buildBlocksRegistry(tpl)
	require.NoError(t, err)

	gatherer := &fakeIssueGatherer{}
	runtime := &fakeDailyRuntime{result: &agent.RunResult{Text: "wrote it", Turns: 2}}
	stdout := &bytes.Buffer{}

	deps := dailyDeps{
		cfg:           cfg,
		template:      tpl,
		registry:      reg,
		scorer:        fixtureScorer{},
		issueGatherer: gatherer,
		runtime:       runtime,
		store:         store,
		voice:         voice.Voice{Enabled: true, Signature: false},
		now:           func() time.Time { return time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC) },
		stdout:        stdout,
	}
	return deps, gatherer, runtime, stdout
}

// simulateAgentWrite wires runtime.onRun to actually create the file
// runDaily's post-run verification expects, mimicking what a real MCP
// archy_write_vault_note tool call would produce. Tests that exercise
// the success path of a non-dry-run runDaily must call this so the
// stat check finds the file.
func simulateAgentWrite(t *testing.T, deps dailyDeps, runtime *fakeDailyRuntime) {
	t.Helper()
	date := deps.now().Format("2006-01-02")
	rel := filepath.Join(deps.cfg.Vault.Folders.Daily, date+".md")
	abs := filepath.Join(deps.cfg.Vault.Path, rel)
	runtime.onRun = func(_ agent.RunRequest) {
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte("test brief\n"), 0o644))
	}
}

func TestRunDaily_FixtureIssues_ProducesNonEmptyBody(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	simulateAgentWrite(t, deps, runtime)
	gatherer.issues = []domain.Issue{
		{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "Fix the thing", Priority: domain.PriorityUrgent},
		{Ref: domain.ExternalRef{Provider: "linear", ID: "SOC-2"}, Title: "Review checklist", Priority: domain.PriorityMedium},
	}

	res, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Body)
	assert.Contains(t, res.Body, "ENG-1")
	assert.Contains(t, res.Body, "Fix the thing")
	assert.Contains(t, res.Body, "Suggested Plan")

	// Agent was invoked exactly once with a prompt that contains both
	// the body and the target path.
	require.Len(t, runtime.calls, 1)
	assert.Equal(t, "daily-brief", runtime.calls[0].SkillName)
	assert.Contains(t, runtime.prompts[0], "ENG-1")
	assert.Contains(t, runtime.prompts[0], "Daily/2026-05-07.md")
}

func TestRunDaily_DryRunPrintsToStdout_NoAgent(t *testing.T) {
	deps, gatherer, runtime, stdout := fixtureDeps(t)
	gatherer.issues = []domain.Issue{
		{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "Hello", Priority: domain.PriorityUrgent},
	}

	res, err := runDaily(context.Background(), deps, dailyOptions{DryRun: true})
	require.NoError(t, err)
	assert.Empty(t, runtime.calls, "dry-run must not invoke the agent")
	assert.Contains(t, stdout.String(), "ENG-1")
	assert.NotEmpty(t, res.Body)
	assert.Nil(t, res.AgentResult)
}

func TestRunDaily_DryRun_EmptyIssuesStillRenders(t *testing.T) {
	deps, _, _, stdout := fixtureDeps(t)
	// gatherer.issues left nil

	res, err := runDaily(context.Background(), deps, dailyOptions{DryRun: true})
	require.NoError(t, err)
	// top_priorities is unavailable when there are no issues, so the
	// body is the synthesis block alone covering the empty case.
	assert.Contains(t, res.Body, "Nothing pressing today")
	assert.Contains(t, stdout.String(), "Nothing pressing today")
}

func TestRunDaily_AgentRuntimeError_WrapsContext(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}
	runtime.err = errors.New("agent SDK exploded")

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily: agent run")
	assert.Contains(t, err.Error(), "agent SDK exploded")
}

func TestRunDaily_IdempotencyClaim_SecondCallSkips(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	simulateAgentWrite(t, deps, runtime)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}

	// First run: fresh claim, agent invoked.
	res1, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	assert.False(t, res1.Skipped)
	assert.Len(t, runtime.calls, 1)

	// Second run on same date: idempotency claim returns false, run is
	// skipped, agent is NOT invoked again.
	res2, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	assert.True(t, res2.Skipped)
	assert.Contains(t, res2.SkipReason, "already")
	assert.Len(t, runtime.calls, 1, "second run must not invoke the agent")
}

func TestRunDaily_GatherFailure(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.err = errors.New("linear unreachable")

	_, err := runDaily(context.Background(), deps, dailyOptions{DryRun: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear unreachable")
	assert.Empty(t, runtime.calls)
}

// Failure path: gather fails → no claim is taken, next run can retry.
func TestRunDaily_GatherFailureLeavesNoClaim(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.err = errors.New("linear unreachable")

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	assert.Empty(t, runtime.calls)

	now := deps.now()
	key := fmt.Sprintf("daily-brief:%s", now.Format("2006-01-02"))
	has, err := deps.store.IdempotencyHas(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, has, "failed gather must not take an idempotency claim")
}

// Failure path: agent run fails → no claim is taken.
func TestRunDaily_AgentFailureLeavesNoClaim(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}
	runtime.err = errors.New("agent SDK exploded")

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)

	now := deps.now()
	key := fmt.Sprintf("daily-brief:%s", now.Format("2006-01-02"))
	has, err := deps.store.IdempotencyHas(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, has, "failed agent run must not take an idempotency claim")
}

// Success path: claim is taken AFTER the agent run.
func TestRunDaily_SuccessTakesClaim(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	simulateAgentWrite(t, deps, runtime)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	require.Len(t, runtime.calls, 1)

	now := deps.now()
	key := fmt.Sprintf("daily-brief:%s", now.Format("2006-01-02"))
	has, err := deps.store.IdempotencyHas(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, has, "successful run must take an idempotency claim")
}

// --force bypasses an existing claim and re-runs.
func TestRunDaily_ForceIgnoresExistingClaim(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	simulateAgentWrite(t, deps, runtime)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}

	// First run: success, takes a claim.
	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	require.Len(t, runtime.calls, 1)

	// Second run with --force: not skipped, invokes the agent again.
	res2, err := runDaily(context.Background(), deps, dailyOptions{Force: true})
	require.NoError(t, err)
	assert.False(t, res2.Skipped)
	assert.Len(t, runtime.calls, 2, "--force must re-invoke the agent")
}

// --force on a day with no prior claim still runs cleanly.
func TestRunDaily_ForceWithoutPriorClaim(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	simulateAgentWrite(t, deps, runtime)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}

	res, err := runDaily(context.Background(), deps, dailyOptions{Force: true})
	require.NoError(t, err)
	assert.False(t, res.Skipped)
	assert.Len(t, runtime.calls, 1)
}

// --force + --dry-run: Force is ignored, dry-run wins (no idempotency
// reads or writes).
func TestRunDaily_ForceAndDryRunCombination(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "X"}, Title: "x"}}

	res, err := runDaily(context.Background(), deps, dailyOptions{DryRun: true, Force: true})
	require.NoError(t, err)
	assert.Empty(t, runtime.calls)
	assert.NotEmpty(t, res.Body)

	now := deps.now()
	key := fmt.Sprintf("daily-brief:%s", now.Format("2006-01-02"))
	has, err := deps.store.IdempotencyHas(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, has, "dry-run must never touch idempotency state")
}

func TestBuildDailyPrompt_IncludesFields(t *testing.T) {
	got := buildDailyPrompt(
		"## Top Priorities\n\n- [ENG-1] hi",
		map[string]any{"title": "Daily Brief 2026-05-07", "date": "2026-05-07"},
		"Daily/2026-05-07.md",
	)
	assert.Contains(t, got, "Daily Brief 2026-05-07")
	assert.Contains(t, got, "Date: 2026-05-07")
	assert.Contains(t, got, "Daily/2026-05-07.md")
	assert.Contains(t, got, "ENG-1")
}

func TestBuildBlock_TopPriorities_ConfiguresLimit(t *testing.T) {
	b, err := buildBlock(render.BlockSpec{Name: "top_priorities", Config: map[string]any{"limit": 3}})
	require.NoError(t, err)
	tp, ok := b.(blocks.TopPrioritiesBlock)
	require.True(t, ok)
	assert.Equal(t, 3, tp.Limit)
}

func TestBuildBlock_Synthesis_ConfiguresStyle(t *testing.T) {
	b, err := buildBlock(render.BlockSpec{Name: "synthesis", Config: map[string]any{"style": "detailed"}})
	require.NoError(t, err)
	sb, ok := b.(blocks.SynthesisBlock)
	require.True(t, ok)
	assert.Equal(t, "detailed", sb.Style)
}

func TestBuildBlock_Unknown(t *testing.T) {
	_, err := buildBlock(render.BlockSpec{Name: "made-up"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown block")
}

// Success path: when the agent actually creates the expected file,
// the post-run verification passes and runDaily returns success.
func TestRunDaily_VerifiesFileAfterRun(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	simulateAgentWrite(t, deps, runtime)
	runtime.result = &agent.RunResult{
		ToolCalls: []agent.ToolCallRecord{
			{Name: "mcp__archy__archy_write_vault_note"},
		},
	}

	res, err := runDaily(context.Background(), deps, dailyOptions{})
	require.NoError(t, err)
	assert.NotNil(t, res.AgentResult)
}

// Missing file with no tool calls: error includes the path, the
// "zero tool calls" call-out, the stat error, the SDK signal line
// (turns/duration/cost), and an explicit "<empty>" marker for the
// missing text so the operator can tell signal from noise.
func TestRunDaily_FailsLoudlyWhenFileMissing(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	// Agent claims success but doesn't write anything and produces
	// no text either.
	runtime.result = &agent.RunResult{ToolCalls: nil, Text: ""}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "no file at")
	assert.Contains(t, msg, "zero tool calls")
	assert.Contains(t, msg, "turns=0")
	assert.Contains(t, msg, "agent said: <empty>")
}

// Missing file with tool calls: every tool call's name and outcome
// shows up in the error so the operator can see what happened. With
// non-empty text, the agent-said line appears too.
func TestRunDaily_MissingFileIncludesToolCallNames(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	runtime.result = &agent.RunResult{
		ToolCalls: []agent.ToolCallRecord{
			{Name: "mcp__archy__archy_score_items"},
			{Name: "Bash", Error: "command failed"},
		},
		Text: "I attempted the score step but bash failed.",
	}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "2 tool call")
	assert.Contains(t, msg, "archy_score_items")
	assert.Contains(t, msg, "Bash")
	assert.Contains(t, msg, "command failed")
	assert.Contains(t, msg, "agent said:")
	assert.Contains(t, msg, "I attempted the score step")
}

// Zero tool calls + agent text: the diagnostic must include what
// the agent actually said so the operator can see why no tool was
// invoked.
func TestRunDaily_MissingFileIncludesAgentText(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	runtime.result = &agent.RunResult{
		ToolCalls: nil,
		Text:      "I cannot access any tools that would let me write to your vault.",
	}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "zero tool calls")
	assert.Contains(t, msg, "agent said:")
	assert.Contains(t, msg, "cannot access any tools")
}

// Long agent text is truncated so the error message stays bounded.
func TestRunDaily_AgentTextTruncated(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	long := strings.Repeat("x", 1200)
	runtime.result = &agent.RunResult{Text: long}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "more chars")
	assert.Less(t, len(msg), 2000, "error message should be bounded")
}

// Turns, duration, and cost from the SDK are surfaced so the operator
// can tell whether the model was consulted at all when both tool
// calls and text are empty.
func TestRunDaily_MissingFileIncludesTurnsAndDuration(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
	gatherer.issues = []domain.Issue{{Ref: domain.ExternalRef{Provider: "linear", ID: "ENG-1"}, Title: "x"}}

	runtime.result = &agent.RunResult{
		Turns:    3,
		Duration: 2500 * time.Millisecond,
		CostUSD:  0.0042,
	}

	_, err := runDaily(context.Background(), deps, dailyOptions{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "turns=3")
	assert.Contains(t, msg, "duration=2.5s")
	assert.Contains(t, msg, "cost_usd=0.004200")
	assert.Contains(t, msg, "agent said: <empty>")
}

func TestSummarizeToolCalls(t *testing.T) {
	assert.Equal(t, "0 tool calls", summarizeToolCalls(nil))
	assert.Equal(t, "1 tool call(s): mcp__archy__archy_write_vault_note",
		summarizeToolCalls([]agent.ToolCallRecord{{Name: "mcp__archy__archy_write_vault_note"}}))
	assert.Equal(t, "2 tool call(s): a, b",
		summarizeToolCalls([]agent.ToolCallRecord{{Name: "a"}, {Name: "b"}}))
}
