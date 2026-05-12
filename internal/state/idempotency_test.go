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

func TestIdempotencyHas_FreshKey(t *testing.T) {
	s := openTestStore(t)
	has, err := s.IdempotencyHas(context.Background(), "daily-brief:2026-05-10")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestIdempotencyHas_ClaimedKey(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	_, err := s.IdempotencyClaim(ctx, "daily-brief:2026-05-10", time.Now())
	require.NoError(t, err)

	has, err := s.IdempotencyHas(ctx, "daily-brief:2026-05-10")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestIdempotencyClear_RemovesClaim(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	_, err := s.IdempotencyClaim(ctx, "daily-brief:2026-05-10", time.Now())
	require.NoError(t, err)

	require.NoError(t, s.IdempotencyClear(ctx, "daily-brief:2026-05-10"))

	has, err := s.IdempotencyHas(ctx, "daily-brief:2026-05-10")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestIdempotencyClear_AbsentKeyNoOp(t *testing.T) {
	s := openTestStore(t)
	require.NoError(t, s.IdempotencyClear(context.Background(), "daily-brief:never-existed"))
}

func TestIdempotencyClear_AllowsReclaim(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	key := "daily-brief:2026-05-10"

	ok, err := s.IdempotencyClaim(ctx, key, time.Now())
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, s.IdempotencyClear(ctx, key))

	ok, err = s.IdempotencyClaim(ctx, key, time.Now())
	require.NoError(t, err)
	assert.True(t, ok, "claim should be fresh again after clear")
}
