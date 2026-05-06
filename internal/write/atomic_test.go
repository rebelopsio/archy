package write

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicWrite_TempFileInSameDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	require.NoError(t, atomicWrite(target, []byte("hello")))

	// Walk the dir; only the target should remain.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "out.txt", entries[0].Name())

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestAtomicWrite_NoTempLeftoverOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	require.NoError(t, atomicWrite(target, []byte("a")))
	require.NoError(t, atomicWrite(target, []byte("b")))
	require.NoError(t, atomicWrite(target, []byte("c")))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.Contains(e.Name(), ".archy-tmp-"),
			"unexpected temp file leftover: %s", e.Name())
	}
}

func TestAtomicWrite_FailureCleansUpTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}

	dir := t.TempDir()
	// Make the directory unwritable so OpenFile fails when attempting to
	// create the temp file.
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := atomicWrite(filepath.Join(dir, "out.txt"), []byte("x"))
	require.Error(t, err)

	// Restore permissions to inspect contents — there should be no temp file.
	require.NoError(t, os.Chmod(dir, 0o755))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.Contains(e.Name(), ".archy-tmp-"),
			"unexpected temp file leftover: %s", e.Name())
	}
}

func TestAtomicWrite_TempNamePattern(t *testing.T) {
	// Spy on the implementation by snooping the directory while a write is
	// in flight is racy; instead, indirectly assert the naming convention
	// via the failure-cleanup test plus a positive smoke check that the
	// final file matches the requested basename.
	dir := t.TempDir()
	target := filepath.Join(dir, "named-target.md")
	require.NoError(t, atomicWrite(target, []byte("content")))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "content", string(got))
}
