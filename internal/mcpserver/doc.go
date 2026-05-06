// Package mcpserver implements archy's MCP server.
//
// Per ADR-0003, archy's deterministic tools live behind an MCP server
// that runs as a subprocess of the agent host (specifically, the same
// archy binary invoked with the hidden "mcp-server" subcommand). This
// package owns the [Server] type, the typed tool handlers, and the
// stdio/in-memory transports used in production and tests respectively.
//
// The server uses github.com/modelcontextprotocol/go-sdk to register
// typed tool handlers; the SDK infers JSON Schema from input/output
// struct tags. The Claude Agent SDK is not imported here — that is the
// agent runtime's concern.
//
// v1 tool surface: archy_write_vault_note, archy_read_vault_note,
// archy_score_items. State-backed tools (carryover, dedupe, explanation
// persistence) and the rendering tool come in a follow-up PRD.
package mcpserver
