package agent

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	claude "github.com/partio-io/claude-agent-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/config"
)

// fakeRunner is the in-package test double that replaces the real SDK
// session driver. Tests hand it a script of messages plus an optional
// terminal error; the iter.Seq2 it returns yields each message in order.
type fakeRunner struct {
	messages    []claude.Message
	terminal    error
	prompt      string
	optsCount   int
	gotOpts     []claude.Option
	calls       int
	beforeYield func(ctx context.Context) // optional ctx observation between yields
}

func (f *fakeRunner) run(ctx context.Context, prompt string, opts []claude.Option) iter.Seq2[claude.Message, error] {
	f.prompt = prompt
	f.optsCount = len(opts)
	f.gotOpts = opts
	f.calls++
	return func(yield func(claude.Message, error) bool) {
		for _, m := range f.messages {
			if f.beforeYield != nil {
				f.beforeYield(ctx)
			}
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}
			if !yield(m, nil) {
				return
			}
		}
		if f.terminal != nil {
			yield(nil, f.terminal)
		}
	}
}

// newTestRuntime returns a Runtime with a baseline-valid config and a
// substituted fake runner. The caller drives the fakeRunner's messages.
func newTestRuntime(t *testing.T, fr *fakeRunner) *Runtime {
	t.Helper()
	rt, err := New(Options{
		Config:          baselineConfig(),
		ArchyBinaryPath: "/fake/archy",
		UserEmail:       "user@example.com",
		UserUsername:    "user",
	})
	require.NoError(t, err)
	rt.runner = fr
	return rt
}

func baselineConfig() *config.Config {
	return &config.Config{
		Vault: config.VaultConfig{
			Path:    "/tmp/vault",
			Folders: config.VaultFolders{Daily: "Daily", Meetings: "Meetings", Triage: "Triage", Reviews: "Reviews", Inbox: "Inbox"},
		},
		Skills: config.SkillsConfig{ProjectDir: ".claude/skills", UserDir: "/home/user/.claude/skills"},
		Agent: config.AgentConfig{
			Model:          "claude-sonnet-4-5",
			MaxTurns:       30,
			PermissionMode: "acceptEdits",
		},
		State:  config.StateConfig{SQLitePath: "/tmp/state.db", CacheTTL: 15 * time.Minute},
		Output: config.OutputConfig{DefaultWriteMode: "marker-block", Timezone: "Local"},
	}
}

func TestNew_MissingConfig(t *testing.T) {
	_, err := New(Options{UserEmail: "u@e", UserUsername: "u"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSetup))
}

func TestNew_MissingUserEmail(t *testing.T) {
	_, err := New(Options{Config: baselineConfig(), ArchyBinaryPath: "/fake/archy", UserUsername: "u"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSetup))
}

func TestNew_MissingUserUsername(t *testing.T) {
	_, err := New(Options{Config: baselineConfig(), ArchyBinaryPath: "/fake/archy", UserEmail: "u@e"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSetup))
}

func TestNew_DefaultsArchyBinaryPathFromExecutable(t *testing.T) {
	rt, err := New(Options{
		Config:       baselineConfig(),
		UserEmail:    "u@e",
		UserUsername: "u",
		// ArchyBinaryPath intentionally empty — should fall back to
		// os.Executable() (the test binary's path).
	})
	require.NoError(t, err)
	assert.NotEmpty(t, rt.opts.ArchyBinaryPath)
}

func TestClose_Idempotent(t *testing.T) {
	rt, err := New(Options{Config: baselineConfig(), ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.NoError(t, err)
	require.NoError(t, rt.Close())
	require.NoError(t, rt.Close())
	require.NoError(t, rt.Close())
}
