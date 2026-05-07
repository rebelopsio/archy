package linear

import "errors"

// ErrConfig is returned by [New] when its [Config] is invalid.
var ErrConfig = errors.New("invalid linear client config")

// ErrToolCall is returned when a Linear MCP tool call fails: either a
// transport error, or a tool-level error response (CallToolResult.IsError).
var ErrToolCall = errors.New("linear tool call failed")

// ErrParse is returned when a Linear MCP response cannot be parsed
// into the expected shape.
var ErrParse = errors.New("linear response parse error")
