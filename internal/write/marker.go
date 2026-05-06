package write

import (
	"fmt"
	"strings"
)

// Canonical marker delimiters. The serializer always produces these exact
// byte sequences and the parser only recognizes these exact byte sequences.
// Whitespace variations are intentionally not tolerated so that a hand-edited
// marker comment fails to match rather than silently mismatching the wrong
// region of the file.
const (
	markerStartPrefix = "<!-- archy:start id="
	markerStartSuffix = " -->"
	markerEnd         = "<!-- archy:end -->"
)

// ValidateMarkerID returns nil when id is a valid archy marker id and an
// error wrapping ErrInvalidMarkerID otherwise. Valid ids match the pattern
// [a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9] (or a single [a-zA-Z0-9] character):
// non-empty, ASCII alphanumerics and hyphens, no leading or trailing hyphen.
//
// Implemented with a hand-rolled loop to avoid pulling in the regexp package.
func ValidateMarkerID(id string) error {
	if id == "" {
		return fmt.Errorf("validate marker id %q: %w", id, ErrInvalidMarkerID)
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-':
			if i == 0 || i == len(id)-1 {
				return fmt.Errorf("validate marker id %q: %w", id, ErrInvalidMarkerID)
			}
		default:
			return fmt.Errorf("validate marker id %q: %w", id, ErrInvalidMarkerID)
		}
	}
	return nil
}

// serializeMarkerBlock returns the canonical marker block for the given id
// and content. The id must already have been validated by the caller.
//
// Content is normalized so the block always ends with exactly one "\n"
// before the end comment: any number of trailing "\n"s on the input
// collapse to one, and content with no trailing newline gains one. Empty
// content produces a block whose body is a single "\n".
func serializeMarkerBlock(id, content string) string {
	body := strings.TrimRight(content, "\n")
	var sb strings.Builder
	sb.Grow(len(markerStartPrefix) + len(id) + len(markerStartSuffix) + 1 + len(body) + 1 + len(markerEnd) + 1)
	sb.WriteString(markerStartPrefix)
	sb.WriteString(id)
	sb.WriteString(markerStartSuffix)
	sb.WriteByte('\n')
	sb.WriteString(body)
	sb.WriteByte('\n')
	sb.WriteString(markerEnd)
	return sb.String()
}
