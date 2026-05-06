package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools registers archy's v1 tool surface on the underlying
// [mcp.Server]. Called from [New] after the server is constructed; tools
// must be registered before [Server.Serve] is called.
func (s *Server) registerTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "archy_write_vault_note",
		Description: "Write a markdown note to the user's vault. " +
			"Use marker-block mode (default) to update only archy-managed regions of files. " +
			"Use overwrite or append for capture-style workflows.",
	}, s.handleWriteVaultNote)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "archy_read_vault_note",
		Description: "Read the contents of a note in the user's vault.",
	}, s.handleReadVaultNote)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "archy_score_items",
		Description: "Rank a list of items (issues, pull requests, calendar events) " +
			"by computed priority. Returns scored items in descending order with " +
			"per-signal explanations.",
	}, s.handleScoreItems)
}

// toolError returns a [mcp.CallToolResult] marking the tool call as
// errored with the given message. Use this for "the tool ran and decided
// the call cannot succeed" — the agent will see the message verbatim.
// Use a regular Go error return only for protocol-level failures the
// agent should not interpret.
func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}
