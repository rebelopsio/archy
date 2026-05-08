package mcpserver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/write"
)

func TestNew_ErrInvalidConfig_NilWriter(t *testing.T) {
	_, err := New(Config{User: domain.MakeIdentity([]string{"u@e.com"}, "u", "u")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
}

func TestNew_ErrInvalidConfig_EmptyUserEmails(t *testing.T) {
	dir := t.TempDir()
	w, err := write.New(dir)
	require.NoError(t, err)
	// Identity with no emails is rejected; handles alone aren't enough.
	_, err = New(Config{Writer: w, User: domain.MakeIdentity(nil, "u", "u")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "User.Emails")
}

func TestServerName(t *testing.T) {
	assert.Equal(t, "archy", ServerName)
}

// TestServer_InMemoryTransport_WriteThroughWire is the one test that
// exercises the wire boundary: a real client and a real server connected
// via in-memory transports, calling archy_write_vault_note over MCP.
func TestServer_InMemoryTransport_WriteThroughWire(t *testing.T) {
	srv, dir := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientT, serverT := mcp.NewInMemoryTransports()

	// Server side: connect first (per SDK convention).
	serverSess, err := srv.mcp.Connect(ctx, serverT, nil)
	require.NoError(t, err)
	defer func() { _ = serverSess.Close() }()

	// Client side.
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSess, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err)
	defer func() { _ = clientSess.Close() }()

	res, err := clientSess.CallTool(ctx, &mcp.CallToolParams{
		Name: "archy_write_vault_note",
		Arguments: map[string]any{
			"path":      "note.md",
			"marker_id": "test",
			"content":   "hello via MCP",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "tool reported error: %v", res.Content)

	// File should now exist on disk.
	assert.FileExists(t, filepath.Join(dir, "note.md"))
}

func TestServer_Serve_CancellationStopsCleanly(t *testing.T) {
	srv, _ := newTestServer(t)
	clientT, serverT := mcp.NewInMemoryTransports()

	// Hold the client side open so the server has a peer; cancel the
	// context to assert Serve unblocks promptly.
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "stalled-client", Version: "v0"}, nil)
		_, _ = client.Connect(context.Background(), clientT, nil)
	}()

	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, serverT) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Either context.Canceled propagates, or the SDK reports a
		// graceful stop. Either is acceptable; the contract is "Serve
		// returns promptly after ctx.Done()".
		_ = err
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of ctx cancellation")
	}
}
