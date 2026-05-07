// Package voice owns archy's personification surface.
//
// Per ADR-0001 decision 10, archy has a name (always lowercase) and a
// thin personality. The personality lives in user-facing output —
// progress messages and the optional "— archy" signature on
// generated marker blocks — but it is suppressed for machine-readable
// outputs (--quiet, --json, error messages, archy doctor, logs).
//
// This package owns the boundary. [Voice.Progress] returns a
// formatted progress message when Enabled, or the empty string when
// disabled (callers suppress output). [Voice.Sign] returns the
// signature line or empty.
package voice
