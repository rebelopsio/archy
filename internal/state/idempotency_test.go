package state

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotency_FirstClaimReturnsTrue(t *testing.T) {
	s := openTestStore(t)
	ok, err := s.IdempotencyClaim(context.Background(), "daily-brief:2026-05-06", time.Now())
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIdempotency_SecondClaimReturnsFalse(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ok, err := s.IdempotencyClaim(ctx, "daily-brief:2026-05-06", time.Now())
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = s.IdempotencyClaim(ctx, "daily-brief:2026-05-06", time.Now())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIdempotency_DifferentKeysIndependent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ok, err := s.IdempotencyClaim(ctx, "daily-brief:2026-05-06", time.Now())
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.IdempotencyClaim(ctx, "daily-brief:2026-05-07", time.Now())
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.IdempotencyClaim(ctx, "meeting-prep:abc123", time.Now())
	require.NoError(t, err)
	assert.True(t, ok)
}
