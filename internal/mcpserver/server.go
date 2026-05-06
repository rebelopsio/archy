package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rebelopsio/archy/internal/scoring"
	"github.com/rebelopsio/archy/internal/write"
)

// ServerName is the stable name used to address archy's MCP server.
// From the agent's perspective, archy's tools appear as
// "mcp__archy__<tool-name>".
const ServerName = "archy"

// serverVersion is the implementation version reported in MCP handshake.
// Bumped on tool-surface changes; the agent does not read it for
// behavior.
const serverVersion = "0.1.0"

// Server is archy's MCP server. It wraps an [mcp.Server] from the
// official SDK and owns references to the underlying packages whose
// functionality it exposes as tools.
type Server struct {
	cfg Config
	mcp *mcp.Server
}

// Config configures a [Server]. All required fields must be set.
type Config struct {
	// Writer handles vault writes. Required.
	Writer *write.Writer

	// ScoringWeights are passed to scoring on each archy_score_items
	// call. Required.
	ScoringWeights scoring.Weights

	// ScoringThresholds tunes signal-firing windows. Optional;
	// scoring.DefaultThresholds() is used field-by-field for any zero
	// values at evaluation time.
	ScoringThresholds scoring.Thresholds

	// UserEmail is the operating user's email. Required.
	UserEmail string

	// UserUsername is the operating user's provider handle. Required.
	UserUsername string

	// KeyStakeholders are usernames or emails whose calendar events get
	// a stakeholder boost. Optional.
	KeyStakeholders []string
}

// New constructs a [Server] from cfg and registers the v1 tools. It
// does not start the server; call [Server.Serve] with a transport to
// begin handling requests. Returns [ErrInvalidConfig] (wrapped) when
// required fields are missing.
func New(cfg Config) (*Server, error) {
	if cfg.Writer == nil {
		return nil, fmt.Errorf("%w: Writer is required", ErrInvalidConfig)
	}
	if cfg.UserEmail == "" {
		return nil, fmt.Errorf("%w: UserEmail is required", ErrInvalidConfig)
	}
	if cfg.UserUsername == "" {
		return nil, fmt.Errorf("%w: UserUsername is required", ErrInvalidConfig)
	}

	s := &Server{
		cfg: cfg,
		mcp: mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: serverVersion}, nil),
	}
	s.registerTools()
	return s, nil
}

// Serve runs the MCP server using transport. It blocks until ctx is
// cancelled or the transport's session ends (e.g., the peer closes the
// connection). Production callers pass [NewStdioTransport]; tests pass
// one half of [mcp.NewInMemoryTransports].
func (s *Server) Serve(ctx context.Context, transport mcp.Transport) error {
	sess, err := s.mcp.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("mcpserver: connect: %w", err)
	}
	defer func() { _ = sess.Close() }()

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// NewStdioTransport returns an MCP transport bound to os.Stdin and
// os.Stdout. Used by the archy mcp-server subcommand to speak the MCP
// protocol with its parent (the claude CLI).
func NewStdioTransport() mcp.Transport {
	return &mcp.StdioTransport{}
}
