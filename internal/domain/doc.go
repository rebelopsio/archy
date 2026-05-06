// Package domain holds archy's provider-agnostic core types.
//
// Per ADR-0002 this package is a leaf: it imports nothing else from
// internal/ and contains stable Go types plus pure functions over them.
// Provider quirks live in adapter packages; the domain stays clean.
package domain
