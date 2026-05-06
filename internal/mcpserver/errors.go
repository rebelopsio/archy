package mcpserver

import "errors"

// ErrInvalidConfig is returned by [New] when its [Config] is missing
// required fields. Most other error paths surface as MCP tool-level
// errors via [mcp.CallToolResult.IsError] rather than Go errors.
var ErrInvalidConfig = errors.New("invalid mcpserver config")
