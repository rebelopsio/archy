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
	"path/filepath"
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
type fakeDailyRuntime struct {
	calls   []agent.RunRequest
	result  *agent.RunResult
	err     error
	closed  bool
	prompts []string
}

func (f *fakeDailyRuntime) Run(_ context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	f.calls = append(f.calls, req)
	f.prompts = append(f.prompts, req.Prompt)
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

func TestRunDaily_FixtureIssues_ProducesNonEmptyBody(t *testing.T) {
	deps, gatherer, runtime, _ := fixtureDeps(t)
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
