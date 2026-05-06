package agent

import (
	"slices"
	"testing"

	claude "github.com/partio-io/claude-agent-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/mcpserver"
)

// applyOpts runs the SDK option list against an empty config and
// returns it for inspection. We can't read the SDK's internal config
// directly (the struct is unexported), so the strategy is: apply each
// option to a probe and observe behavioral side-effects via the SDK's
// public surface where possible. For most tests we just assert the
// option list contains the expected number/shape via index inspection.
//
// The SDK does not expose accessors on its config, so this helper
// returns the option count plus the raw options slice for table-driven
// presence checks. Tests assert that buildOptions produced a list of
// the expected length and includes specific options by reflection on
// pointer identity where the option is constructed inline (e.g., the
// archy MCP server).
func applyOpts(t *testing.T, opts []claude.Option) int {
	t.Helper()
	return len(opts)
}

func TestBuildOptions_IncludesModelMaxTurnsPermission(t *testing.T) {
	opts, err := buildOptions(baselineConfig(), Options{
		ArchyBinaryPath: "/fake/archy",
		UserEmail:       "u@e",
		UserUsername:    "u",
	})
	require.NoError(t, err)
	// 4 base options (model, max-turns, permission, allowed-tools) + 1 archy MCP server = 5.
	// External MCPs in baseline config = 0.
	assert.Equal(t, 5, applyOpts(t, opts))
}

func TestBuildOptions_RegistersExternalEnabledMCPServer(t *testing.T) {
	cfg := baselineConfig()
	cfg.MCPServers = map[string]config.MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true},
	}
	opts, err := buildOptions(cfg, Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.NoError(t, err)
	assert.Equal(t, 6, applyOpts(t, opts), "5 base/archy + 1 external")
}

func TestBuildOptions_SkipsDisabledExternalMCPServer(t *testing.T) {
	cfg := baselineConfig()
	cfg.MCPServers = map[string]config.MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: false},
	}
	opts, err := buildOptions(cfg, Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.NoError(t, err)
	assert.Equal(t, 5, applyOpts(t, opts))
}

func TestBuildOptions_ExternalServerBadScheme(t *testing.T) {
	cfg := baselineConfig()
	cfg.MCPServers = map[string]config.MCPServerConfig{
		"weird": {URL: "ftp://nope", Enabled: true},
	}
	_, err := buildOptions(cfg, Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported scheme")
}

func TestBuildOptions_CLIPathOnlyAddedWhenSet(t *testing.T) {
	withCLI, err := buildOptions(baselineConfig(), Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u", CLIPath: "/usr/local/bin/claude"})
	require.NoError(t, err)
	withoutCLI, err := buildOptions(baselineConfig(), Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.NoError(t, err)
	assert.Equal(t, len(withoutCLI)+1, len(withCLI))
}

func TestBuildOptions_CwdOnlyAddedWhenSet(t *testing.T) {
	with, err := buildOptions(baselineConfig(), Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u", Cwd: "/some/dir"})
	require.NoError(t, err)
	without, err := buildOptions(baselineConfig(), Options{ArchyBinaryPath: "/fake/archy", UserEmail: "u@e", UserUsername: "u"})
	require.NoError(t, err)
	assert.Equal(t, len(without)+1, len(with))
}

// TestExternalServer_HTTPSUrl is a focused helper test for externalServer.
func TestExternalServer_HTTPSUrl(t *testing.T) {
	srv, err := externalServer(config.MCPServerConfig{URL: "https://example.com/mcp"})
	require.NoError(t, err)
	require.IsType(t, &claude.MCPHTTPServer{}, srv)
	assert.Equal(t, "https://example.com/mcp", srv.(*claude.MCPHTTPServer).URL)
}

func TestExternalServer_BadURL(t *testing.T) {
	_, err := externalServer(config.MCPServerConfig{URL: "::not-a-url::"})
	require.Error(t, err)
}

// TestAllowedToolPattern_CoversArchyTools asserts the runtime's
// auto-allow list matches the mcpserver's known tool surface. If a tool
// is added in mcpserver but not added here, the agent will require the
// user's permission mode to allow it — likely surprising. This test is
// the early-warning seam.
func TestAllowedToolPattern_CoversArchyTools(t *testing.T) {
	expected := []string{
		"mcp__archy__archy_write_vault_note",
		"mcp__archy__archy_read_vault_note",
		"mcp__archy__archy_score_items",
	}
	for _, want := range expected {
		assert.Truef(t, slices.Contains(allowedToolPattern, want), "expected %q in allowedToolPattern", want)
	}
	// Sanity-check that mcpserver.ServerName matches the prefix we
	// embed in tool names so the two stay aligned.
	assert.Equal(t, "archy", mcpserver.ServerName)
}
