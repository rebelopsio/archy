// Package render walks a template and composes the markdown body that
// archy writes to the vault. A template is a YAML file declaring which
// [blocks.Block] to invoke and how each is configured. The renderer
// gathers and renders each block in order, joining outputs with one
// blank line between non-empty sections.
//
// Per ADR-0001 decision 6, the block-based template model exists
// because pure string templating becomes unreadable as soon as more
// than two providers feed a section. A block whose Available returns
// false is silently skipped — graceful degradation is a first-class
// concern, not an afterthought.
//
// The renderer is partial-failure-tolerant: a block that errors at
// Gather or Render is recorded in [BlockResult] but the rest of the
// template continues. The returned error is non-nil when any block
// errored, but the partial body is also returned. Per the project
// instructions, "a partial brief is better than total failure."
//
// Per ADR-0002 the package imports only [internal/blocks] and the
// standard library plus gopkg.in/yaml.v3 for template parsing.
package render
