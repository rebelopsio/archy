package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rebelopsio/archy/internal/write"
)

// WriteVaultNoteInput is the typed input for archy_write_vault_note.
// The MCP SDK infers JSON Schema from these struct tags.
type WriteVaultNoteInput struct {
	// Path is the target. Required.
	Path string `json:"path" jsonschema:"vault-relative or absolute path. must resolve inside the vault"`
	// Mode is the write mode. Optional; defaults to "marker-block".
	Mode string `json:"mode,omitempty" jsonschema:"write mode: marker-block (default), overwrite, or append"`
	// MarkerID is the marker block id. Required when Mode is "marker-block".
	MarkerID string `json:"marker_id,omitempty" jsonschema:"marker block id. required when mode is marker-block"`
	// Content is the body to write. Required.
	Content string `json:"content" jsonschema:"content to write"`
	// Frontmatter is YAML frontmatter applied only on file creation.
	Frontmatter map[string]any `json:"frontmatter,omitempty" jsonschema:"yaml frontmatter for new files. ignored if the file already exists"`
}

// WriteVaultNoteOutput is the typed output for archy_write_vault_note.
type WriteVaultNoteOutput struct {
	// Path is the absolute path that was written.
	Path string `json:"path"`
	// Created is true if the file did not exist before this call.
	Created bool `json:"created"`
	// BlockAdded is true if a marker block was added that didn't exist.
	BlockAdded bool `json:"block_added"`
	// BlockUpdated is true if an existing marker block was replaced.
	BlockUpdated bool `json:"block_updated"`
	// BytesWritten is the resulting file size, or 0 when no write occurred.
	BytesWritten int `json:"bytes_written"`
}

// handleWriteVaultNote dispatches to internal/write. Writer-level errors
// (path escape, duplicate marker, invalid marker id, etc.) surface as
// MCP tool-level errors with the writer's message; the agent reads them.
func (s *Server) handleWriteVaultNote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in WriteVaultNoteInput,
) (*mcp.CallToolResult, WriteVaultNoteOutput, error) {
	mode, err := parseWriteMode(in.Mode)
	if err != nil {
		return toolError(err.Error()), WriteVaultNoteOutput{}, nil
	}

	res, err := s.cfg.Writer.Write(ctx, write.Note{
		Path:        in.Path,
		Mode:        mode,
		MarkerID:    in.MarkerID,
		Content:     in.Content,
		Frontmatter: in.Frontmatter,
	})
	if err != nil {
		return toolError(err.Error()), WriteVaultNoteOutput{}, nil
	}

	return nil, WriteVaultNoteOutput{
		Path:         res.Path,
		Created:      res.Created,
		BlockAdded:   res.BlockAdded,
		BlockUpdated: res.BlockUpdated,
		BytesWritten: res.BytesWritten,
	}, nil
}

// parseWriteMode maps the JSON mode string onto [write.Mode]. An empty
// string defaults to ModeMarkerBlock.
func parseWriteMode(s string) (write.Mode, error) {
	switch s {
	case "", "marker-block":
		return write.ModeMarkerBlock, nil
	case "overwrite":
		return write.ModeOverwrite, nil
	case "append":
		return write.ModeAppend, nil
	default:
		return 0, &writeModeError{mode: s}
	}
}

// writeModeError reports an unknown write mode supplied by the agent.
type writeModeError struct{ mode string }

func (e *writeModeError) Error() string {
	return "unknown write mode " + e.mode + " (expected marker-block, overwrite, or append)"
}
