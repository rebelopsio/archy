package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleWriteVaultNote_Success(t *testing.T) {
	srv, dir := newTestServer(t)
	res, out, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:     "note.md",
		MarkerID: "daily-brief",
		Content:  "today",
	})
	require.NoError(t, err)
	assert.Nil(t, res, "no tool-level error expected")
	assert.True(t, out.Created)
	assert.True(t, out.BlockAdded)
	assert.False(t, out.BlockUpdated)
	assert.Equal(t, filepath.Join(dir, "note.md"), out.Path)
	assert.Greater(t, out.BytesWritten, 0)
}

func TestHandleWriteVaultNote_DefaultModeIsMarkerBlock(t *testing.T) {
	srv, dir := newTestServer(t)
	_, out, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:     "note.md",
		MarkerID: "daily-brief",
		Content:  "hi",
	})
	require.NoError(t, err)
	body, err := os.ReadFile(out.Path)
	require.NoError(t, err)
	assert.Contains(t, string(body), "<!-- archy:start id=daily-brief -->")
	assert.NoFileExists(t, filepath.Join(dir, "doesnt-exist.md"))
}

func TestHandleWriteVaultNote_OverwriteMode(t *testing.T) {
	srv, _ := newTestServer(t)
	_, out, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:    "note.md",
		Mode:    "overwrite",
		Content: "raw content",
	})
	require.NoError(t, err)
	body, err := os.ReadFile(out.Path)
	require.NoError(t, err)
	assert.Equal(t, "raw content\n", string(body))
}

func TestHandleWriteVaultNote_AppendMode(t *testing.T) {
	srv, _ := newTestServer(t)
	_, _, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:    "log.md",
		Mode:    "append",
		Content: "first\n",
	})
	require.NoError(t, err)
	_, out, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:    "log.md",
		Mode:    "append",
		Content: "second\n",
	})
	require.NoError(t, err)

	body, err := os.ReadFile(out.Path)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(body))
}

func TestHandleWriteVaultNote_UnknownMode_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:    "note.md",
		Mode:    "bogus",
		Content: "x",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	require.Len(t, res.Content, 1)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, text, "unknown write mode")
}

func TestHandleWriteVaultNote_PathOutsideVault_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	other := t.TempDir()
	res, _, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:     filepath.Join(other, "leak.md"),
		MarkerID: "x",
		Content:  "no",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.True(t, strings.Contains(text, "escapes vault root") || strings.Contains(text, "path escape"),
		"expected escape message, got %q", text)
}

func TestHandleWriteVaultNote_InvalidMarkerID_ToolError(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _, err := srv.handleWriteVaultNote(context.Background(), &mcp.CallToolRequest{}, WriteVaultNoteInput{
		Path:     "note.md",
		MarkerID: "bad id with spaces",
		Content:  "hi",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
}
