package domain

import "time"

// PriorityScore is the scoring engine's output for a single ranked item.
// Items in a sorted list are sorted descending by Score.
type PriorityScore struct {
	// Ref identifies the item that was scored.
	Ref ExternalRef

	// Score is the total computed priority. Higher is more urgent.
	Score int

	// Signals is the list of named signals that contributed to the
	// score, with their individual contributions. Used by --explain
	// and the in-process MCP server's record_explanation tool.
	Signals []ScoreSignal

	// ComputedAt is when the score was calculated. Useful for debugging
	// stale rankings.
	ComputedAt time.Time
}

// ScoreSignal is one signal that contributed to a PriorityScore.
type ScoreSignal struct {
	// Name is a short identifier, e.g. "meeting_soon", "urgent_issue".
	// Used as a config key for weights and as a display label.
	Name string

	// Weight is the configured weight for this signal at scoring time.
	Weight int

	// Triggered is true if the signal fired (contributed Weight to the
	// score). False signals are recorded too, so --explain can show
	// what was checked and why it didn't apply.
	Triggered bool

	// Reason is a one-line human-readable explanation of why the signal
	// did or didn't fire. Optional.
	Reason string
}
