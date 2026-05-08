package agent

import (
	"fmt"
	"net/url"
	"strings"

	claude "github.com/partio-io/claude-agent-sdk-go"

	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/mcpserver"
)

// allowedToolPattern is the explicit per-tool allowlist for archy's
// own tools. The SDK's WithAllowedTools matches by exact name; archy's
// tools are deterministic and safe by construction so they auto-approve.
var allowedToolPattern = []string{
	"mcp__archy__archy_write_vault_note",
	"mcp__archy__archy_read_vault_note",
	"mcp__archy__archy_score_items",
}

// buildOptions translates archy config + per-runtime opts into the
// SDK's option list. The list is consumed by both [claude.NewSession]
// and the fake-SDK runner used in tests, so options.go owns the full
// translation in one place.
func buildOptions(cfg *config.Config, opts Options) ([]claude.Option, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: Config is required", ErrSetup)
	}
	if len(opts.User.Emails) == 0 {
		return nil, fmt.Errorf("%w: User.Emails must contain at least one entry", ErrSetup)
	}

	out := []claude.Option{
		claude.WithModel(cfg.Agent.Model),
		claude.WithMaxTurns(cfg.Agent.MaxTurns),
		claude.WithPermissionMode(cfg.Agent.PermissionMode),
		claude.WithAllowedTools(allowedToolPattern...),
	}
	if opts.CLIPath != "" {
		out = append(out, claude.WithCLIPath(opts.CLIPath))
	}
	if opts.Cwd != "" {
		out = append(out, claude.WithCwd(opts.Cwd))
	}

	out = append(out, claude.WithMCPServer(mcpserver.ServerName, &claude.MCPStdioServer{
		Command: opts.ArchyBinaryPath,
		Args:    []string{"mcp-server"},
		Env: map[string]string{
			"ARCHY_USER_EMAILS":        strings.Join(opts.User.Emails, ","),
			"ARCHY_USER_LINEAR_HANDLE": opts.User.LinearHandle,
			"ARCHY_USER_GITHUB_HANDLE": opts.User.GitHubHandle,
		},
	}))

	for name, srv := range cfg.MCPServers {
		if !srv.Enabled {
			continue
		}
		ext, err := externalServer(srv)
		if err != nil {
			return nil, fmt.Errorf("%w: mcp_servers[%q]: %v", ErrSetup, name, err)
		}
		out = append(out, claude.WithMCPServer(name, ext))
	}

	return out, nil
}

// externalServer maps a config.MCPServerConfig to the SDK's MCP server
// type. The current schema doesn't distinguish HTTP vs SSE explicitly,
// so v1 defaults to HTTP. SSE-only servers will need a config schema
// addition.
func externalServer(srv config.MCPServerConfig) (claude.MCPServerConfig, error) {
	u, err := url.Parse(srv.URL)
	if err != nil {
		return nil, fmt.Errorf("parse url %q: %w", srv.URL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q (expected http or https)", u.Scheme)
	}
	return &claude.MCPHTTPServer{URL: srv.URL}, nil
}
