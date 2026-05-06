// Package scoring is archy's deterministic priority engine.
//
// The engine takes a slice of items (issues, pull requests, calendar events)
// and a closed set of named, weighted signals, and produces a ranked list
// of [domain.PriorityScore] values. It is a pure function: no I/O, no
// time.Now, no LLM. The reference time, user identity, and weights are all
// passed in by the caller.
//
// The agent calls this package via the in-process MCP server's
// archy_score_items tool. The LLM may explain or interpret the resulting
// scores, but the ranking itself is computed here.
//
// Per ADR-0002 this package is a leaf: it imports only [internal/domain]
// and the standard library.
package scoring
