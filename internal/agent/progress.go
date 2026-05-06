package agent

import "time"

// ProgressEvent is a single observation about a running skill. The
// runtime emits these during [Runtime.Run] for callers (CLI, future
// TUI, future Neovim plugin) that want to render live status.
type ProgressEvent struct {
	// Kind categorizes the event.
	Kind ProgressKind
	// Message is a human-readable description; for ProgressToolCall it
	// also encodes the tool name. May be empty for some kinds.
	Message string
	// ToolName is set when Kind is [ProgressToolCall].
	ToolName string
	// At is when the event was observed.
	At time.Time
}

// ProgressKind enumerates progress event categories.
type ProgressKind int

// ProgressKind values.
const (
	// ProgressUnknown is the zero value.
	ProgressUnknown ProgressKind = iota
	// ProgressStart fires once when the agent run begins.
	ProgressStart
	// ProgressToolCall fires for each tool the agent invokes.
	ProgressToolCall
	// ProgressTextChunk fires for assistant-text content blocks.
	ProgressTextChunk
	// ProgressTurnComplete fires when the SDK reports the run's result
	// message — informally a "turn completed" signal.
	ProgressTurnComplete
	// ProgressEnd fires once when Run is about to return.
	ProgressEnd
)

// String returns the lowercase, snake_case form of the kind. Out-of-range
// values return "unknown".
func (k ProgressKind) String() string {
	switch k {
	case ProgressStart:
		return "start"
	case ProgressToolCall:
		return "tool_call"
	case ProgressTextChunk:
		return "text_chunk"
	case ProgressTurnComplete:
		return "turn_complete"
	case ProgressEnd:
		return "end"
	case ProgressUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}
