package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/scoring"
	"github.com/rebelopsio/archy/internal/write"
)

// newTestServer constructs a Server with a real Writer rooted at a
// fresh tempdir. Returns the server and the vault root.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := write.New(dir)
	require.NoError(t, err)

	srv, err := New(Config{
		Writer:         w,
		ScoringWeights: scoring.Weights{UrgentIssue: 8, OverdueIssue: 5, MeetingSoon: 3, ReviewRequested: 7, BlockedOnUser: 6, CIFailing: 4},
		User:           domain.MakeIdentity([]string{"user@example.com"}, "user", "user"),
	})
	require.NoError(t, err)
	return srv, dir
}
