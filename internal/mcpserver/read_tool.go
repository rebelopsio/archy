package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rebelopsio/archy/internal/write"
)

// ReadVaultNoteInput is the typed input for archy_read_vault_note.
type ReadVaultNoteInput struct {
	// Path is the target. Required.
	Path string `json:"path" jsonschema:"vault-relative or absolute path. must resolve inside the vault"`
}

// ReadVaultNoteOutput is the typed output for archy_read_vault_note.
type ReadVaultNoteOutput struct {
	// Path is the absolute path that was read.
	Path string `json:"path"`
	// Content is the file's contents as a UTF-8 string.
	Content string `json:"content"`
	// Size is the byte length of Content.
	Size int `json:"size"`
}

// handleReadVaultNote reads a note's contents after path validation
// matching the writer's escape check. The writer does not expose a
// public Read method — reading is a distinct concern that may evolve
// separately. The path-resolution logic is duplicated here (see
// [resolveReadPath]); a parameterized test asserts the two
// implementations agree.
func (s *Server) handleReadVaultNote(
	_ context.Context,
	_ *mcp.CallToolRequest,
	in ReadVaultNoteInput,
) (*mcp.CallToolResult, ReadVaultNoteOutput, error) {
	abs, err := resolveReadPath(s.cfg.Writer.VaultRoot, in.Path)
	if err != nil {
		return toolError(err.Error()), ReadVaultNoteOutput{}, nil
	}

	data, err := os.ReadFile(abs) //nolint:gosec // abs is validated against vault root
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return toolError(fmt.Sprintf("note not found: %s", in.Path)), ReadVaultNoteOutput{}, nil
		}
		return toolError(err.Error()), ReadVaultNoteOutput{}, nil
	}

	return nil, ReadVaultNoteOutput{
		Path:    abs,
		Content: string(data),
		Size:    len(data),
	}, nil
}

// resolveReadPath validates p against vaultRoot using the same escape
// check as the writer: clean, absolute-or-vault-relative, reject if the
// resulting path is outside the (cleaned) vault root. Returns the
// cleaned absolute path on success or an error whose message mentions
// path escape on rejection.
func resolveReadPath(vaultRoot, p string) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(vaultRoot, p))
	}
	vaultClean := filepath.Clean(vaultRoot)
	if abs == vaultClean {
		return "", fmt.Errorf("read %s: %w", p, write.ErrPathEscape)
	}
	rel, err := filepath.Rel(vaultClean, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("read %s: %w", p, write.ErrPathEscape)
	}
	return abs, nil
}
