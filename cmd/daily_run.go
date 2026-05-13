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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rebelopsio/archy/internal/agent"
	"github.com/rebelopsio/archy/internal/blocks"
	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/render"
	"github.com/rebelopsio/archy/internal/scoring"
	"github.com/rebelopsio/archy/internal/state"
	"github.com/rebelopsio/archy/internal/voice"
)

// IssueGatherer abstracts the data-fetching step so tests can supply
// fixtures and so future providers (GitHub, calendars) can satisfy
// the same shape. Per ADR-0006 this is the orchestrator path: the Go
// side fetches Linear data directly and feeds the renderer.
type IssueGatherer interface {
	GatherIssues(ctx context.Context) ([]domain.Issue, error)
}

// dailyRuntime is the unexported seam that lets tests substitute the
// agent runtime. The real *agent.Runtime satisfies this directly;
// the fake in daily_run_test.go records the prompt and yields scripted
// results.
type dailyRuntime interface {
	Run(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error)
	Close() error
}

// dailyDeps bundles the runtime collaborators runDaily accepts. Tests
// pass fakes for the IssueGatherer and dailyRuntime; the cobra command
// constructs production wiring.
type dailyDeps struct {
	cfg           *config.Config
	template      *render.Template
	registry      *blocks.Registry
	scorer        blocks.ItemScorer
	issueGatherer IssueGatherer
	runtime       dailyRuntime
	store         *state.Store
	voice         voice.Voice
	now           func() time.Time
	stdout        io.Writer
}

// dailyOptions controls per-invocation behavior.
type dailyOptions struct {
	// DryRun renders the brief and prints it to deps.stdout without
	// invoking the agent or writing to the vault.
	DryRun bool
	// Force causes runDaily to bypass the idempotency-has check and
	// clear any prior claim for today, allowing regeneration of an
	// already-written brief. Has no effect when DryRun is true.
	Force bool
}

// dailyResult is what runDaily returns on success. The cobra wrapper
// uses this to print the final confirmation; tests assert on it.
type dailyResult struct {
	// Body is the rendered markdown. Always populated.
	Body string
	// Frontmatter includes runtime-computed fields plus any from the
	// template.
	Frontmatter map[string]any
	// TargetPath is the vault-relative path the agent was instructed
	// to write to.
	TargetPath string
	// AgentResult is non-nil after a non-dry-run invocation; nil on
	// dry-run or when idempotency caused an early skip.
	AgentResult *agent.RunResult
	// Skipped is true iff idempotency claimed prevented this run.
	Skipped bool
	// SkipReason describes why the run was skipped (idempotency).
	SkipReason string
}

// runDaily orchestrates a daily-brief execution. The caller wires
// production deps; tests inject fakes. Behavior:
//
//  1. Check for a prior idempotency claim under the run's date key;
//     skip when one exists (skipped entirely on dry-run).
//  2. Gather issues via deps.issueGatherer.
//  3. Build a GatherContext and drive the renderer.
//  4. Dry-run: print body to stdout and return.
//  5. Real run: invoke the agent with a prompt that includes the body
//     and target path. The agent calls archy_write_vault_note.
//  6. Only after the agent run succeeds: take the idempotency claim.
//     Failures before this point leave no claim, so the next attempt
//     can retry without manual intervention.
func runDaily(ctx context.Context, deps dailyDeps, opts dailyOptions) (dailyResult, error) {
	now := deps.now()
	date := now.Format("2006-01-02")
	targetPath := filepath.Join(deps.cfg.Vault.Folders.Daily, date+".md")
	key := fmt.Sprintf("daily-brief:%s", date)

	if !opts.DryRun {
		if opts.Force {
			if err := deps.store.IdempotencyClear(ctx, key); err != nil {
				return dailyResult{}, fmt.Errorf("daily: idempotency clear: %w", err)
			}
		} else {
			has, err := deps.store.IdempotencyHas(ctx, key)
			if err != nil {
				return dailyResult{}, fmt.Errorf("daily: idempotency check: %w", err)
			}
			if has {
				return dailyResult{
					Skipped:    true,
					SkipReason: "today's brief has already been generated",
					TargetPath: targetPath,
				}, nil
			}
		}
	}

	issues, err := deps.issueGatherer.GatherIssues(ctx)
	if err != nil {
		return dailyResult{}, fmt.Errorf("daily: gather issues: %w", err)
	}

	sources := map[string]struct{}{}
	for name, srv := range deps.cfg.MCPServers {
		if srv.Enabled {
			sources[name] = struct{}{}
		}
	}

	gctx := blocks.GatherContext{
		Now:     now,
		Sources: sources,
		Issues:  issues,
		Scorer:  deps.scorer,
	}

	renderer := render.NewRenderer(deps.registry)
	res, renderErr := renderer.Render(ctx, deps.template, gctx)
	// Per the renderer's partial-failure contract, an error means at
	// least one block failed but the partial body is still returned.
	// Surface the error to the caller via voice/log; do not bail.
	body := res.Body + deps.voice.Sign()

	if opts.DryRun {
		_, _ = fmt.Fprintln(deps.stdout, body)
		if renderErr != nil {
			_, _ = fmt.Fprintf(deps.stdout, "\n# render warnings:\n%s\n", renderErr.Error())
		}
		return dailyResult{
			Body:        body,
			Frontmatter: res.Frontmatter,
			TargetPath:  targetPath,
		}, nil
	}

	prompt := buildDailyPrompt(body, res.Frontmatter, targetPath)
	runRes, err := deps.runtime.Run(ctx, agent.RunRequest{
		SkillName: "daily-brief",
		Prompt:    prompt,
	})
	if err != nil {
		return dailyResult{}, fmt.Errorf("daily: agent run: %w", err)
	}

	// Claim is taken only after the agent run returns success, so a
	// failed gather/render/agent invocation leaves no claim behind and
	// the next archy daily can retry. A "false" fresh result here is
	// benign and only reachable via --force on a date with a prior
	// claim (which the caller cleared just before this run started).
	if _, err := deps.store.IdempotencyClaim(ctx, key, now); err != nil {
		return dailyResult{}, fmt.Errorf("daily: idempotency claim: %w", err)
	}

	// Verify the file actually landed. The agent can report success
	// without having invoked the write tool — surface that loudly with
	// the list of tool calls the agent did make. The claim survives
	// this failure on purpose; re-run with --force to retry.
	absPath := filepath.Join(deps.cfg.Vault.Path, targetPath)
	info, statErr := os.Stat(absPath)
	if statErr != nil || info.Size() == 0 {
		return dailyResult{}, fmt.Errorf(
			"daily: agent claimed success but no file at %s: %w",
			absPath,
			explainAgentOutcome(runRes, statErr),
		)
	}

	return dailyResult{
		Body:        body,
		Frontmatter: res.Frontmatter,
		TargetPath:  targetPath,
		AgentResult: runRes,
	}, nil
}

// explainAgentOutcome returns an error summarizing what the agent did
// (and didn't do) so the operator can diagnose why the expected file
// wasn't written. Surfaces tool calls, turns/duration/cost from the
// SDK, and the agent's text output. Empty text is reported as
// "<empty>" explicitly because it's a meaningful signal, not noise.
func explainAgentOutcome(res *agent.RunResult, statErr error) error {
	var b strings.Builder
	if len(res.ToolCalls) == 0 {
		fmt.Fprintf(&b, "agent made zero tool calls (stat: %v)", statErr)
	} else {
		fmt.Fprintf(&b, "agent made %d tool call(s):", len(res.ToolCalls))
		for i, c := range res.ToolCalls {
			outcome := "ok"
			if c.Error != "" {
				outcome = "error: " + c.Error
			}
			fmt.Fprintf(&b, "\n  [%d] %s — %s", i+1, c.Name, outcome)
		}
		fmt.Fprintf(&b, "\n(stat: %v)", statErr)
	}
	fmt.Fprintf(&b, "\nturns=%d duration=%s cost_usd=%.6f",
		res.Turns, res.Duration, res.CostUSD)
	if res.Text != "" {
		fmt.Fprintf(&b, "\nagent said: %s", truncateText(res.Text, 1000))
	} else {
		fmt.Fprintf(&b, "\nagent said: <empty>")
	}
	if res.SubprocessStderr != "" {
		fmt.Fprintf(&b, "\nclaude stderr: %s", truncateText(res.SubprocessStderr, 2000))
	}
	return errors.New(b.String())
}

// truncateText clips s to n chars and appends a "(N more chars)"
// marker when truncated. Local to the cmd package because the
// internal/linear truncate is package-private and only one caller
// needs this here.
func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("…(%d more chars)", len(s)-n)
}

// summarizeToolCalls returns a one-line description of the tool calls
// the agent made, suitable for the success-path "wrote <path>" line.
// Zero tool calls is reported explicitly because it almost always
// means the agent didn't do what was asked.
func summarizeToolCalls(calls []agent.ToolCallRecord) string {
	if len(calls) == 0 {
		return "0 tool calls"
	}
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	return fmt.Sprintf("%d tool call(s): %s", len(calls), strings.Join(names, ", "))
}

// buildDailyPrompt is the prompt the daily-brief skill receives. Skill
// prose tells the agent to write the body to TargetPath using the
// supplied frontmatter; the prose lives in SKILL.md and references
// these fields by name.
func buildDailyPrompt(body string, frontmatter map[string]any, targetPath string) string {
	var sb strings.Builder
	sb.WriteString("Write today's daily brief.\n\n")
	fmt.Fprintf(&sb, "Target path: %s\n", targetPath)
	if t, ok := frontmatter["title"].(string); ok {
		fmt.Fprintf(&sb, "Title: %s\n", t)
	}
	if d, ok := frontmatter["date"].(string); ok {
		fmt.Fprintf(&sb, "Date: %s\n", d)
	}
	sb.WriteString("\nBody:\n")
	sb.WriteString(body)
	return sb.String()
}

// buildBlocksRegistry constructs the registry that holds the concrete
// block instances configured by the template. Each spec's Config map
// drives the corresponding block's per-invocation fields. Unknown
// block names produce an error.
func buildBlocksRegistry(tpl *render.Template) (*blocks.Registry, error) {
	reg := blocks.NewRegistry()
	for _, spec := range tpl.Blocks {
		b, err := buildBlock(spec)
		if err != nil {
			return nil, err
		}
		if err := reg.Register(b); err != nil {
			return nil, fmt.Errorf("register %q: %w", spec.Name, err)
		}
	}
	return reg, nil
}

// buildBlock dispatches on spec.Name to construct the right concrete
// block with config applied.
func buildBlock(spec render.BlockSpec) (blocks.Block, error) {
	switch spec.Name {
	case "top_priorities":
		b := blocks.TopPrioritiesBlock{}
		if v, ok := spec.Config["limit"].(int); ok {
			b.Limit = v
		}
		return b, nil
	case "synthesis":
		b := blocks.SynthesisBlock{}
		if v, ok := spec.Config["style"].(string); ok {
			b.Style = v
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unknown block %q in template", spec.Name)
	}
}

// runScorer satisfies blocks.ItemScorer by delegating to scoring.ScoreAll.
type runScorer struct {
	ctx scoring.Context
}

// ScoreIssues wraps each issue in a scoring.IssueItem and runs the
// engine. Returned slice is sorted by the engine.
func (s runScorer) ScoreIssues(_ context.Context, issues []domain.Issue) []domain.PriorityScore {
	items := make([]scoring.Item, 0, len(issues))
	for _, iss := range issues {
		items = append(items, scoring.IssueItem{Issue: iss})
	}
	return scoring.ScoreAll(s.ctx, items)
}
