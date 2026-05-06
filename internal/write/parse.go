// Package write puts bytes on disk for archy-managed vault notes.
//
// The package is the single point in archy responsible for translating a
// generated note into a file on the user's filesystem. It is consumed by
// the in-process MCP server's write_vault_note tool. archy notes coexist
// with hand-authored notes and with notes synced by the Obsidian mobile
// app; the writer therefore must:
//
//   - Be atomic (tmpfile + fsync + rename in the same directory).
//   - Be marker-block-aware: archy only owns content between
//     "<!-- archy:start id=... -->" and "<!-- archy:end -->" comments.
//   - Be idempotent: rerunning the same write produces the same bytes.
//
// # Line endings
//
// archy always writes "\n" line endings. The parser tolerates "\r\n" in
// the input (treats it as "\n" for fence and marker matching). When a
// marker block is updated, the content between the markers is normalized
// to "\n". Content outside marker blocks is preserved byte-for-byte —
// including its line endings — except for the very last byte of the file,
// which is normalized to a single trailing "\n".
//
// # Code fences
//
// Marker comments inside fenced code blocks ("```" or "~~~") are not
// treated as markers. Indented code blocks (4+ leading spaces) are not
// detected in v1; if you need to embed a literal marker comment inside
// an indented block, wrap it in a fenced block instead.
package write

import (
	"bytes"
	"fmt"
	"strings"
)

// parsedFile is the structured view of a markdown file used by the writer.
// It owns offsets into the original raw bytes so writers can splice in a
// new marker block while preserving everything outside byte-for-byte.
type parsedFile struct {
	raw    []byte
	blocks []parsedBlock
}

// parsedBlock describes one archy marker block found in raw bytes.
//
// Byte ranges are absolute offsets into parsedFile.raw:
//
//	[start, end)                 — the entire block, including the
//	                               start and end comment lines and their
//	                               trailing newlines (or end of input).
//	[contentStart, contentEnd)   — the bytes between the marker lines,
//	                               i.e. the block's body.
type parsedBlock struct {
	id           string
	start        int
	end          int
	contentStart int
	contentEnd   int
}

// findBlock returns a pointer to the first block matching id, or nil.
func (pf *parsedFile) findBlock(id string) *parsedBlock {
	for i := range pf.blocks {
		if pf.blocks[i].id == id {
			return &pf.blocks[i]
		}
	}
	return nil
}

// parse tokenizes raw into marker blocks. It returns ErrUnclosedMarker for
// a start without a matching end (or vice versa) and ErrDuplicateMarker if
// two blocks share an id. A file with no marker blocks parses successfully
// and yields an empty blocks slice.
func parse(raw []byte) (*parsedFile, error) {
	pf := &parsedFile{raw: raw}
	var fence fenceInfo
	var pending *parsedBlock
	var pendingHeaderEnd int
	seen := make(map[string]struct{})

	lineStart := 0
	for lineStart < len(raw) {
		nl := bytes.IndexByte(raw[lineStart:], '\n')
		var lineEnd, nextLineStart int
		if nl < 0 {
			lineEnd = len(raw)
			nextLineStart = len(raw)
		} else {
			absoluteNl := lineStart + nl
			nextLineStart = absoluteNl + 1
			lineEnd = absoluteNl
			if lineEnd > lineStart && raw[lineEnd-1] == '\r' {
				lineEnd--
			}
		}
		line := string(raw[lineStart:lineEnd])

		prevOpen := fence.open
		wasDelimiter := fence.consume(line)

		if !prevOpen && !wasDelimiter {
			if id, ok := parseStartLine(line); ok {
				if pending != nil {
					return nil, fmt.Errorf("parse: %w (start at offset %d)", ErrUnclosedMarker, pending.start)
				}
				pending = &parsedBlock{id: id, start: lineStart}
				pendingHeaderEnd = nextLineStart
				lineStart = nextLineStart
				continue
			}
			if line == markerEnd {
				if pending == nil {
					return nil, fmt.Errorf("parse: %w (end at offset %d)", ErrUnclosedMarker, lineStart)
				}
				pending.contentStart = pendingHeaderEnd
				pending.contentEnd = lineStart
				pending.end = nextLineStart
				if _, dup := seen[pending.id]; dup {
					return nil, fmt.Errorf("parse marker id %q: %w", pending.id, ErrDuplicateMarker)
				}
				seen[pending.id] = struct{}{}
				pf.blocks = append(pf.blocks, *pending)
				pending = nil
				lineStart = nextLineStart
				continue
			}
		}

		lineStart = nextLineStart
	}

	if pending != nil {
		return nil, fmt.Errorf("parse: %w (start at offset %d)", ErrUnclosedMarker, pending.start)
	}
	return pf, nil
}

// parseStartLine returns (id, true) if line is a canonical archy:start
// marker comment with a valid id, or ("", false) otherwise. The parser
// recognizes only the canonical form so that hand-edited markers fail
// loudly instead of silently mismatching the wrong region.
func parseStartLine(line string) (string, bool) {
	if !strings.HasPrefix(line, markerStartPrefix) {
		return "", false
	}
	if !strings.HasSuffix(line, markerStartSuffix) {
		return "", false
	}
	id := line[len(markerStartPrefix) : len(line)-len(markerStartSuffix)]
	if err := ValidateMarkerID(id); err != nil {
		return "", false
	}
	return id, true
}

// fenceInfo tracks whether the parser is currently inside a fenced code
// block. CommonMark allows up to 3 leading spaces of indent, an opening
// fence of 3+ backticks or tildes, and a closing fence of 3+ of the same
// character with a run length at least as long as the opener and no info
// string after the fence characters.
type fenceInfo struct {
	open   bool
	char   byte
	minLen int
}

// consume updates f for one line of input and returns true if the line
// was itself a fence delimiter (an opener or a closer). A delimiter line
// is never eligible to be a marker comment.
func (f *fenceInfo) consume(line string) bool {
	indent := 0
	for indent < 4 && indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent >= 4 {
		return false
	}
	rest := line[indent:]
	if len(rest) < 3 {
		return false
	}
	c := rest[0]
	if c != '`' && c != '~' {
		return false
	}
	runLen := 0
	for runLen < len(rest) && rest[runLen] == c {
		runLen++
	}
	if runLen < 3 {
		return false
	}
	info := rest[runLen:]

	if !f.open {
		if c == '`' && strings.ContainsRune(info, '`') {
			return false
		}
		f.open = true
		f.char = c
		f.minLen = runLen
		return true
	}

	if c != f.char || runLen < f.minLen {
		return false
	}
	if strings.TrimSpace(info) != "" {
		return false
	}
	f.open = false
	return true
}
