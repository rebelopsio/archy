package state

import "time"

// formatTime returns t serialized as RFC3339Nano in UTC. The zero
// time.Time produces an empty string — the convention storage layer uses
// for "no time."
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime parses an RFC3339Nano string back into time.Time. An empty
// string returns the zero time.Time. Both nano and second precision are
// accepted (time.Parse with RFC3339Nano handles both).
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}
