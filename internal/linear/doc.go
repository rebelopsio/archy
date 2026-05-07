// Package linear is archy's direct Go MCP client for Linear.
//
// Per [ADR-0006], archy fetches structured data (Linear issues, GitHub
// PRs, calendar events) directly via Go MCP clients rather than
// through the agent. The agent's job narrows to writing markdown and
// narrating progress. This package implements the Linear half.
//
// Authentication uses a Linear personal access token sourced from an
// environment variable named in [config.MCPServerConfig.BearerTokenEnv].
// The token is passed as "Authorization: Bearer <token>" on every
// outbound HTTP request via a wrapping http.RoundTripper. The MCP Go
// SDK's StreamableClientTransport accepts a custom *http.Client
// directly, so static-bearer auth needs no OAuthHandler ceremony.
//
// v1 surface: [Client.ListMyIssues] returns the calling user's open
// assigned issues. Future workflows extend the surface as they need
// it; do not pre-emptively expose the full Linear MCP tool set.
//
// Probe-derived implementation choices (verified live against
// https://mcp.linear.app/mcp):
//
//   - Tool: list_issues
//   - Input: {"assignee": "me", "limit": 50, "orderBy": "updatedAt"}
//   - completed/canceled state filtering is client-side (server-side
//     state filter wasn't available; pagination is deferred to v2 —
//     >50-issue users see truncated lists in v1)
//   - Priority value semantics are inverted from intuition: 0 = no
//     priority, 1 = urgent (highest), 4 = low (lowest). See [convert.go]
//     for the mapping table.
//   - State decodes from `statusType` (enum), never `status`
//     (display name).
//   - Issue identifier in [domain.ExternalRef.ID] is Linear's
//     human-readable team-prefixed form ("ENG-761"), not a UUID.
package linear
