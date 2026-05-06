package agent

import (
	"context"
	"fmt"
	"iter"
	"os"
	"sync"

	claude "github.com/partio-io/claude-agent-sdk-go"

	"github.com/rebelopsio/archy/internal/config"
)

// Runtime wraps the Claude Agent SDK Go for archy's use.
type Runtime struct {
	cfg  *config.Config
	opts Options

	// runner is the seam tests use to substitute a fake SDK. The real
	// runner constructs a [claude.Session], calls Send, and returns
	// the iter.Seq2 yielded by Stream.
	runner runner

	// closeOnce guards Close from being called multiple times.
	closeOnce sync.Once
}

// Options configures a [Runtime]. UserEmail and UserUsername are
// required; the rest have sensible defaults.
type Options struct {
	// Config is archy's loaded configuration. Required.
	Config *config.Config

	// ArchyBinaryPath is the absolute path to the archy binary. The
	// runtime spawns this binary as a child process via
	// [claude.MCPStdioServer] to expose archy's deterministic tools.
	// If empty, defaults to os.Executable().
	ArchyBinaryPath string

	// CLIPath overrides the path to the claude binary. Optional;
	// if empty, the SDK searches $PATH.
	CLIPath string

	// Cwd is the working directory for the SDK subprocess. Optional;
	// the SDK uses the current working directory when unset.
	Cwd string

	// UserEmail and UserUsername are passed through to the archy MCP
	// server subprocess via environment variables. Required (the
	// scoring engine needs them).
	UserEmail    string
	UserUsername string
}

// New constructs a [Runtime]. Returns [ErrSetup] (wrapped) when
// required Options fields are missing or the archy binary path cannot
// be resolved.
func New(opts Options) (*Runtime, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("%w: Options.Config is required", ErrSetup)
	}

	if opts.ArchyBinaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("%w: locate archy binary: %v", ErrSetup, err)
		}
		opts.ArchyBinaryPath = exe
	}

	if _, err := buildOptions(opts.Config, opts); err != nil {
		return nil, err
	}

	return &Runtime{
		cfg:    opts.Config,
		opts:   opts,
		runner: realRunner{},
	}, nil
}

// Close releases SDK resources. Safe to call multiple times. The real
// SDK session is created and closed per Run, so Close is currently a
// no-op; reserved for future per-Runtime resources (e.g., a long-lived
// claude session if archy ever moves to that mode).
func (r *Runtime) Close() error {
	r.closeOnce.Do(func() {})
	return nil
}

// runner is the unexported seam that lets tests substitute a fake SDK.
// Implementations build a SDK session, send the prompt, and yield
// messages via the iter.Seq2 the SDK exposes.
type runner interface {
	run(ctx context.Context, prompt string, opts []claude.Option) iter.Seq2[claude.Message, error]
}

// realRunner is the production implementation that drives an actual
// [claude.Session]. The Send-then-Stream sequence matches the SDK's
// session-mode example in its package doc.
type realRunner struct{}

func (realRunner) run(ctx context.Context, prompt string, opts []claude.Option) iter.Seq2[claude.Message, error] {
	return func(yield func(claude.Message, error) bool) {
		sess := claude.NewSession(opts...)
		defer func() { _ = sess.Close() }()

		if err := sess.Send(ctx, prompt); err != nil {
			yield(nil, err)
			return
		}
		for msg, err := range sess.Stream(ctx) {
			if !yield(msg, err) {
				return
			}
		}
	}
}
