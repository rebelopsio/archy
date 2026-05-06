package state

import "errors"

// ErrOpen is returned when [Open] cannot create or initialize the
// database. The wrapping error message includes the underlying cause.
var ErrOpen = errors.New("state: open failed")

// ErrNotFound is returned by lookup methods when no row matches the
// requested ref or id.
var ErrNotFound = errors.New("state: not found")
