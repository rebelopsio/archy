// Package agent adapts the Claude Agent SDK Go to archy's needs.
//
// Per [ADR-0001] decision 1, archy is built on
// github.com/partio-io/claude-agent-sdk-go. The SDK wraps the claude
// CLI as a subprocess and provides the agent loop, tool dispatch, MCP
// integration, and subagent orchestration. This package is the single
// seam between archy and that SDK: it builds SDK options from
// [config.Config], wires archy's MCP server (per [ADR-0003]) as an
// [claude.MCPStdioServer] pointing at the same archy binary running
// with the hidden mcp-server subcommand, registers external MCP
// servers, executes a named skill, and translates the SDK's message
// stream into archy [ProgressEvent]s and a typed [RunResult].
//
// Skill invocation: the SDK auto-discovers skills from
// .claude/skills/ (project) and ~/.claude/skills/ (user). To select a
// skill, archy appends a one-line system-prompt instruction via
// [claude.WithAppendSystemPrompt] (see [run.go]).
//
// Cancellation: [Runtime.Run] honors ctx — if cancelled, the SDK's
// session.Stream errors out and Run returns ctx.Err() wrapped, after
// emitting a final ProgressEnd event.
package agent
