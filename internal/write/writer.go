package write

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Mode controls how Write treats the target file.
type Mode int

const (
	// ModeMarkerBlock updates only the content between matching marker
	// comments, preserving everything else. The default for archy.
	ModeMarkerBlock Mode = iota

	// ModeOverwrite replaces the entire file. Used rarely, only when a
	// template explicitly opts in.
	ModeOverwrite

	// ModeAppend appends content to the end of the file without markers.
	// Used for capture-style workflows.
	ModeAppend
)

// Note describes a single write operation.
type Note struct {
	// Path is the target file. May be absolute or relative to the
	// Writer's VaultRoot. Must resolve to a path inside VaultRoot.
	Path string

	// Mode determines write semantics.
	Mode Mode

	// MarkerID identifies the marker block to update. Required when Mode
	// is ModeMarkerBlock; ignored otherwise. Must satisfy ValidateMarkerID.
	MarkerID string

	// Content is the body to write. For ModeMarkerBlock, this is the
	// content between the marker comments (the writer adds the comments
	// themselves). For ModeOverwrite and ModeAppend, this is the literal
	// content written.
	Content string

	// Frontmatter is written only when creating a new file. The map is
	// serialized as YAML in sorted-key order so the output is deterministic.
	// Ignored if the target file already exists.
	Frontmatter map[string]any
}

// Validate returns an error if the Note is malformed. Called automatically
// by Write but exported so callers can pre-validate.
func (n Note) Validate() error {
	if n.Path == "" {
		return errors.New("note: path is required")
	}
	if n.Mode == ModeMarkerBlock {
		if err := ValidateMarkerID(n.MarkerID); err != nil {
			return err
		}
	}
	return nil
}

// Result describes what happened during a Write call.
type Result struct {
	// Path is the absolute path written.
	Path string
	// Created is true if the file did not exist before this write.
	Created bool
	// BlockAdded is true if a marker block was added (didn't exist).
	BlockAdded bool
	// BlockUpdated is true if an existing marker block was replaced.
	BlockUpdated bool
	// BytesWritten is the byte count of the resulting file (or, for
	// ModeAppend, the number of bytes appended). 0 when no write occurred.
	BytesWritten int
}

// Writer writes notes to a vault directory.
type Writer struct {
	// VaultRoot is the absolute path to the user's vault. All writes must
	// resolve to a path inside this directory.
	VaultRoot string

	// Clock is injected for testability. Defaults to time.Now if nil.
	// v1 of the writer does not emit timestamps; the field is reserved
	// for future use (e.g., generated_by frontmatter).
	Clock func() time.Time
}

// New constructs a Writer. vaultRoot must be an absolute path to an
// existing directory. Returns ErrVaultRootInvalid (wrapped) if vaultRoot
// is relative, does not exist, or is not a directory.
func New(vaultRoot string) (*Writer, error) {
	if !filepath.IsAbs(vaultRoot) {
		return nil, fmt.Errorf("new writer %q: %w", vaultRoot, ErrVaultRootInvalid)
	}
	info, err := os.Stat(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("new writer %q: %w", vaultRoot, ErrVaultRootInvalid)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("new writer %q: %w", vaultRoot, ErrVaultRootInvalid)
	}
	return &Writer{VaultRoot: filepath.Clean(vaultRoot)}, nil
}

// Write performs a single note write according to n.Mode. It is safe to
// call concurrently for different files; concurrent writes to the same
// file are not synchronized — the OS rename is atomic, but two concurrent
// writes may race on read-modify-write.
func (w *Writer) Write(ctx context.Context, n Note) (Result, error) {
	if err := n.Validate(); err != nil {
		return Result{}, err
	}
	absPath, err := w.resolvePath(n.Path)
	if err != nil {
		return Result{}, err
	}
	res := Result{Path: absPath}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return res, fmt.Errorf("write %s: ensure parent: %w", absPath, err)
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}

	// Touch Clock so its presence in the code path is observable. The
	// timestamp is currently unused; this also keeps the field non-dead
	// for forward compatibility.
	w.now()

	switch n.Mode {
	case ModeMarkerBlock:
		return w.writeMarkerBlock(absPath, n, res)
	case ModeOverwrite:
		return w.writeOverwrite(absPath, n, res)
	case ModeAppend:
		return w.writeAppend(absPath, n, res)
	default:
		return res, fmt.Errorf("write %s: unknown mode %d", absPath, n.Mode)
	}
}

func (w *Writer) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

func (w *Writer) resolvePath(p string) (string, error) {
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(w.VaultRoot, p))
	}
	vaultClean := filepath.Clean(w.VaultRoot)
	if abs == vaultClean {
		return "", fmt.Errorf("write %s: %w", p, ErrPathEscape)
	}
	rel, err := filepath.Rel(vaultClean, abs)
	if err != nil {
		return "", fmt.Errorf("write %s: %w", p, ErrPathEscape)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("write %s: %w", p, ErrPathEscape)
	}
	return abs, nil
}

func (w *Writer) writeMarkerBlock(absPath string, n Note, res Result) (Result, error) {
	raw, exists, err := readMaybe(absPath)
	if err != nil {
		return res, fmt.Errorf("write %s: %w", absPath, err)
	}
	blockBytes := []byte(serializeMarkerBlock(n.MarkerID, n.Content))

	if !exists {
		var buf []byte
		if len(n.Frontmatter) > 0 {
			fm, err := serializeFrontmatter(n.Frontmatter)
			if err != nil {
				return res, fmt.Errorf("write %s: frontmatter: %w", absPath, err)
			}
			buf = append(buf, fm...)
			buf = append(buf, '\n')
		}
		buf = append(buf, blockBytes...)
		buf = ensureSingleTrailingNewline(buf)
		if err := atomicWrite(absPath, buf); err != nil {
			return res, err
		}
		res.Created = true
		res.BlockAdded = true
		res.BytesWritten = len(buf)
		return res, nil
	}

	pf, err := parse(raw)
	if err != nil {
		return res, fmt.Errorf("write %s: %w", absPath, err)
	}

	if existing := pf.findBlock(n.MarkerID); existing != nil {
		out := make([]byte, 0, len(raw)-(existing.end-existing.start)+len(blockBytes)+1)
		out = append(out, raw[:existing.start]...)
		out = append(out, blockBytes...)
		// serializeMarkerBlock has no trailing newline, but the original
		// block range consumed its trailing newline (or end-of-input).
		// Re-add a newline so the line after the end comment lands where
		// it did before — including any blank-line separators.
		out = append(out, '\n')
		out = append(out, raw[existing.end:]...)
		out = ensureSingleTrailingNewline(out)
		if bytes.Equal(out, raw) {
			return res, nil
		}
		if err := atomicWrite(absPath, out); err != nil {
			return res, err
		}
		res.BlockUpdated = true
		res.BytesWritten = len(out)
		return res, nil
	}

	var out []byte
	trimmed := trimTrailingNewlines(raw)
	if len(trimmed) == 0 {
		out = append(out, blockBytes...)
	} else {
		out = append(out, trimmed...)
		out = append(out, '\n', '\n')
		out = append(out, blockBytes...)
	}
	out = ensureSingleTrailingNewline(out)
	if bytes.Equal(out, raw) {
		return res, nil
	}
	if err := atomicWrite(absPath, out); err != nil {
		return res, err
	}
	res.BlockAdded = true
	res.BytesWritten = len(out)
	return res, nil
}

func (w *Writer) writeOverwrite(absPath string, n Note, res Result) (Result, error) {
	_, exists, err := readMaybe(absPath)
	if err != nil {
		return res, fmt.Errorf("write %s: %w", absPath, err)
	}
	out := ensureSingleTrailingNewline([]byte(n.Content))
	if err := atomicWrite(absPath, out); err != nil {
		return res, err
	}
	res.Created = !exists
	res.BytesWritten = len(out)
	return res, nil
}

func (w *Writer) writeAppend(absPath string, n Note, res Result) (Result, error) {
	existing, exists, err := readMaybe(absPath)
	if err != nil {
		return res, fmt.Errorf("write %s: %w", absPath, err)
	}
	if !exists {
		out := ensureSingleTrailingNewline([]byte(n.Content))
		if err := atomicWrite(absPath, out); err != nil {
			return res, err
		}
		res.Created = true
		res.BytesWritten = len(out)
		return res, nil
	}
	full := append([]byte{}, existing...)
	if len(full) > 0 && full[len(full)-1] != '\n' {
		full = append(full, '\n')
	}
	full = append(full, []byte(n.Content)...)
	full = ensureSingleTrailingNewline(full)
	if err := atomicWrite(absPath, full); err != nil {
		return res, err
	}
	res.BytesWritten = len(full) - len(existing)
	return res, nil
}

// readMaybe returns the file's contents and whether it existed. A
// nonexistent file returns (nil, false, nil).
func readMaybe(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, err
}

// ensureSingleTrailingNewline returns b with all trailing CR/LF bytes
// stripped and exactly one '\n' appended.
func ensureSingleTrailingNewline(b []byte) []byte {
	end := trimTrailingNewlinesIndex(b)
	out := make([]byte, end+1)
	copy(out, b[:end])
	out[end] = '\n'
	return out
}

// trimTrailingNewlines returns b with all trailing CR/LF bytes stripped.
func trimTrailingNewlines(b []byte) []byte {
	return b[:trimTrailingNewlinesIndex(b)]
}

func trimTrailingNewlinesIndex(b []byte) int {
	end := len(b)
	for end > 0 {
		c := b[end-1]
		if c == '\n' || c == '\r' {
			end--
			continue
		}
		break
	}
	return end
}

// serializeFrontmatter returns YAML-framed frontmatter bytes with keys in
// sorted order so the output is deterministic. The result includes the
// opening "---\n" and closing "---\n" delimiters and ends with a single
// trailing "\n". Callers that want a blank line between frontmatter and
// body must add one themselves.
func serializeFrontmatter(fm map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("---\n")
	for _, k := range keys {
		b, err := yaml.Marshal(map[string]any{k: fm[k]})
		if err != nil {
			return nil, err
		}
		sb.Write(b)
	}
	sb.WriteString("---\n")
	return []byte(sb.String()), nil
}
