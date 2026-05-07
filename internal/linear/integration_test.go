//go:build integration

// Tier-2 integration smoke for internal/linear. Skipped from default
// `go test`; CI does not run it. To run manually:
//
//	ARCHY_INTEGRATION_TEST=1 ARCHY_LINEAR_TOKEN=<fresh-PAT> \
//	    go test -tags=integration -run TestListMyIssues_LiveSmoke ./internal/linear/...
//
// Skip conditions: ARCHY_INTEGRATION_TEST != "1", or
// ARCHY_LINEAR_TOKEN unset/empty.
//
// The smoke asserts that calling ListMyIssues against the real
// https://mcp.linear.app/mcp endpoint returns without error and
// produces a (possibly empty) slice. We do not assert on content
// because the user's actual issues are unpredictable; the goal is
// to confirm the wiring works end-to-end.
package linear

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListMyIssues_LiveSmoke(t *testing.T) {
	if os.Getenv("ARCHY_INTEGRATION_TEST") != "1" {
		t.Skip("set ARCHY_INTEGRATION_TEST=1 to run live Linear smoke")
	}
	token := os.Getenv("ARCHY_LINEAR_TOKEN")
	if token == "" {
		t.Skip("ARCHY_LINEAR_TOKEN must be set to a fresh Linear PAT for the live smoke")
	}

	c, err := New(Config{
		URL:         "https://mcp.linear.app/mcp",
		BearerToken: token,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	issues, err := c.ListMyIssues(ctx)
	require.NoError(t, err)

	// Don't assert content; the user's actual issue list is
	// unpredictable. Asserting non-nil-ness confirms the wire path
	// returned cleanly. Issues may legitimately be empty if the user
	// has no open assigned items.
	assert.NotNil(t, issues)
	t.Logf("smoke ok: %d open assigned issue(s)", len(issues))
}
