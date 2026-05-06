package state

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/state/db"
)

func TestCache_PutThenGet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CachePut(ctx, "linear", "issue-1", []byte("hello"), time.Hour))

	val, ok, err := s.CacheGet(ctx, "linear", "issue-1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("hello"), val)
}

func TestCache_GetMissing(t *testing.T) {
	s := openTestStore(t)
	val, ok, err := s.CacheGet(context.Background(), "linear", "missing")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCache_GetExpired_NotDeleted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CachePut(ctx, "p", "k", []byte("v"), -1*time.Second))

	val, ok, err := s.CacheGet(ctx, "p", "k")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, val)

	// The row is still on disk — direct query through the generated
	// layer confirms it (CacheGet itself filters by expiry; we want to
	// verify the row hasn't been deleted).
	q, err := s.queries()
	require.NoError(t, err)
	_, err = q.CacheGet(ctx, db.CacheGetParams{Provider: "p", Key: "k"})
	require.NoError(t, err, "expired entry should still be on disk until vacuum")
}

func TestCache_PutOverwrites(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CachePut(ctx, "p", "k", []byte("first"), time.Hour))
	require.NoError(t, s.CachePut(ctx, "p", "k", []byte("second"), time.Hour))

	val, ok, err := s.CacheGet(ctx, "p", "k")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("second"), val)
}

func TestCache_VacuumDeletesOnlyExpired(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CachePut(ctx, "p", "fresh", []byte("a"), time.Hour))
	require.NoError(t, s.CachePut(ctx, "p", "stale1", []byte("b"), -1*time.Second))
	require.NoError(t, s.CachePut(ctx, "p", "stale2", []byte("c"), -1*time.Second))

	n, err := s.CacheVacuum(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	val, ok, err := s.CacheGet(ctx, "p", "fresh")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("a"), val)
}

func TestCache_VacuumOnCleanCacheReturnsZero(t *testing.T) {
	s := openTestStore(t)
	n, err := s.CacheVacuum(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
