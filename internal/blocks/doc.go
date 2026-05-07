// Package blocks defines archy's block model: the typed units that
// compose a generated note. A workflow's template lists which blocks
// to gather and render; the [Renderer] in internal/render walks the
// template and calls each block's Available, Gather, and Render
// methods in turn.
//
// v1 ships two concrete blocks:
//
//   - [TopPrioritiesBlock] — gathers a ranked subset of the user's
//     issues and renders them as a bulleted list.
//   - [SynthesisBlock] — produces a short narrative summary. v1 is
//     deterministic (rule-driven phrasing); per ADR-0004, real
//     LLM-driven synthesis arrives behind a [Synthesizer] interface
//     in a follow-up PRD.
//
// New block types live alongside these. Per ADR-0002 the package
// imports only [internal/domain] and the standard library.
package blocks
