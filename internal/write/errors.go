package write

import "errors"

// ErrInvalidMarkerID is returned when a caller supplies a marker id that
// contains whitespace, quotes, or other disallowed characters, or that is
// empty. Marker ids must match [a-zA-Z0-9][a-zA-Z0-9-]*.
var ErrInvalidMarkerID = errors.New("invalid marker id")

// ErrPathEscape is returned when a Note.Path resolves to a location outside
// the Writer's VaultRoot. Paths containing ".." that resolve back inside the
// vault are allowed; only paths whose cleaned absolute form is not prefixed
// by the cleaned vault root are rejected.
var ErrPathEscape = errors.New("path escapes vault root")

// ErrDuplicateMarker is returned when a target file contains two or more
// archy marker blocks sharing the same id. The writer refuses to update a
// file in this state because there is no unambiguous block to replace.
var ErrDuplicateMarker = errors.New("duplicate marker id")

// ErrUnclosedMarker is returned when a target file contains an
// "<!-- archy:start id=... -->" without a matching "<!-- archy:end -->",
// or an end without a preceding start.
var ErrUnclosedMarker = errors.New("unclosed marker block")

// ErrVaultRootInvalid is returned by New when the supplied vault root does
// not exist or is not a directory.
var ErrVaultRootInvalid = errors.New("vault root invalid")
