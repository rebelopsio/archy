package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "dirs", "state.db")
	s, err := Open(context.Background(), target)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	info, err := os.Stat(filepath.Dir(target))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	s1, err := Open(context.Background(), path)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	s2, err := Open(context.Background(), path)
	require.NoError(t, err)
	require.NoError(t, s2.Close())
}

func TestOpen_UnwritablePathReturnsErrOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses chmod restrictions")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := Open(context.Background(), filepath.Join(dir, "child", "state.db"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOpen))
}

func TestClose_SafeMultipleTimes(t *testing.T) {
	s := openTestStore(t)
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
}

func TestStore_MethodsAfterCloseReturnError(t *testing.T) {
	s := openTestStore(t)
	require.NoError(t, s.Close())
	err := s.CachePut(context.Background(), "p", "k", []byte("v"), 0)
	require.Error(t, err)
}

func TestStore_ConcurrentReads(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CachePut(ctx, "p", "k", []byte("hello"), 1*time.Hour))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, ok, err := s.CacheGet(ctx, "p", "k")
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, []byte("hello"), val)
		}()
	}
	wg.Wait()
}

func TestStore_ConcurrentWritesDifferentKeys(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	type entry struct {
		key string
		val byte
	}
	entries := []entry{
		{"a", 0}, {"b", 1}, {"c", 2}, {"d", 3}, {"e", 4},
		{"f", 5}, {"g", 6}, {"h", 7}, {"i", 8}, {"j", 9},
	}

	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func(e entry) {
			defer wg.Done()
			err := s.CachePut(ctx, "p", e.key, []byte{e.val}, 1*time.Hour)
			assert.NoError(t, err)
		}(e)
	}
	wg.Wait()

	for _, e := range entries {
		val, ok, err := s.CacheGet(ctx, "p", e.key)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []byte{e.val}, val)
	}
}
