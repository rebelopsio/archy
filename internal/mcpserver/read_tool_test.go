package mcpserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/write"
)

func TestHandleReadVaultNote_Success(t *testing.T) {
	srv, dir := newTestServer(t)
	target := filepath.Join(dir, "note.md")
	require.NoError(t, os.WriteFile(target, []byte("hello"), 0o644))

	res, out, err := srv.handleReadVaultNote(context.Background(), &mcp.CallToolRequest{}, ReadVaultNoteInput{Path: "note.md"})
	require.NoError(t, err)
	assert.Nil(t, res)
	assert.Equal(t, "hello", out.Content)
	assert.Equal(t, 5, out.Size)
	assert.Equal(t, target, out.Path)
}

func TestHandleReadVaultNote_NotFound_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _, err := srv.handleReadVaultNote(context.Background(), &mcp.CallToolRequest{}, ReadVaultNoteInput{Path: "missing.md"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, text, "not found")
}

func TestHandleReadVaultNote_PathOutsideVault_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	other := t.TempDir()
	res, _, err := srv.handleReadVaultNote(context.Background(), &mcp.CallToolRequest{}, ReadVaultNoteInput{Path: filepath.Join(other, "x.md")})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}

// TestResolveReadPath_AgreesWithWriter verifies that the duplicated
// path-validation logic in resolveReadPath behaves the same as the
// writer's internal path validation across a representative set of
// inputs. If they ever diverge, the parameterized comparison surfaces
// the divergence here rather than at runtime.
func TestResolveReadPath_AgreesWithWriter(t *testing.T) {
	dir := t.TempDir()
	w, err := write.New(dir)
	require.NoError(t, err)

	// Build an issue-list of paths that should all fail the same way
	// under both implementations (path escape).
	other := t.TempDir()
	rejectCases := []string{
		filepath.Join(other, "leak.md"), // absolute outside vault
		"../escape.md",                  // relative .. that escapes
		dir,                             // the vault root itself
	}
	for _, p := range rejectCases {
		t.Run("reject_"+p, func(t *testing.T) {
			_, readErr := resolveReadPath(dir, p)
			require.Error(t, readErr)
			require.True(t, errors.Is(readErr, write.ErrPathEscape) || strings.Contains(readErr.Error(), "required"),
				"resolveReadPath for %q: unexpected error %v", p, readErr)

			_, writeErr := w.Write(context.Background(), write.Note{
				Path: p, Mode: write.ModeMarkerBlock, MarkerID: "x", Content: "x",
			})
			require.Error(t, writeErr)
			assert.True(t, errors.Is(writeErr, write.ErrPathEscape) || strings.Contains(writeErr.Error(), "path is required"),
				"writer for %q: unexpected error %v", p, writeErr)
		})
	}

	// Paths that should be accepted by resolveReadPath.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	acceptCases := []string{
		"note.md",
		filepath.Join(dir, "note.md"),
		"sub/../note.md", // .. that resolves inside
	}
	for _, p := range acceptCases {
		t.Run("accept_"+p, func(t *testing.T) {
			_, err := resolveReadPath(dir, p)
			assert.NoError(t, err)
		})
	}
}
