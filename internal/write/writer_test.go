package write

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWriter(t *testing.T) (*Writer, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := New(dir)
	require.NoError(t, err)
	return w, dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestWriter_Write_NewFile_Frontmatter(t *testing.T) {
	w, dir := newTestWriter(t)
	res, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "daily-brief",
		Content:  "hello world",
		Frontmatter: map[string]any{
			"title": "Daily Brief",
			"tag":   "demo",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "note.md"), res.Path)
	assert.True(t, res.Created)
	assert.True(t, res.BlockAdded)
	assert.False(t, res.BlockUpdated)

	expected := "---\ntag: demo\ntitle: Daily Brief\n---\n\n<!-- archy:start id=daily-brief -->\nhello world\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, res.Path))
	assert.Equal(t, len(expected), res.BytesWritten)
}

func TestWriter_Write_NewFile_NoFrontmatter(t *testing.T) {
	w, dir := newTestWriter(t)
	res, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "daily-brief",
		Content:  "hi",
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "note.md"), res.Path)
	assert.True(t, res.Created)
	assert.True(t, res.BlockAdded)

	expected := "<!-- archy:start id=daily-brief -->\nhi\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, res.Path))
}

func TestWriter_Write_ExistingFile_NoMarkerBlock_Appends(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	writeFile(t, target, "# heading\n\nsome paragraph.\n")

	res, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "daily-brief",
		Content:  "today",
	})
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.True(t, res.BlockAdded)
	assert.False(t, res.BlockUpdated)

	expected := "# heading\n\nsome paragraph.\n\n<!-- archy:start id=daily-brief -->\ntoday\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, target))
}

func TestWriter_Write_ExistingMarkerBlock_SameID_Replaces(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	original := "preface\n\n<!-- archy:start id=daily-brief -->\nold content\n<!-- archy:end -->\n\nfooter\n"
	writeFile(t, target, original)

	res, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "daily-brief",
		Content:  "new content",
	})
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.False(t, res.BlockAdded)
	assert.True(t, res.BlockUpdated)

	expected := "preface\n\n<!-- archy:start id=daily-brief -->\nnew content\n<!-- archy:end -->\n\nfooter\n"
	assert.Equal(t, expected, readFile(t, target))
}

func TestWriter_Write_ExistingMarkerBlock_DifferentID_AppendsNew(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	original := "<!-- archy:start id=alpha -->\nA\n<!-- archy:end -->\n"
	writeFile(t, target, original)

	res, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "beta",
		Content:  "B",
	})
	require.NoError(t, err)
	assert.True(t, res.BlockAdded)
	assert.False(t, res.BlockUpdated)

	expected := "<!-- archy:start id=alpha -->\nA\n<!-- archy:end -->\n\n<!-- archy:start id=beta -->\nB\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, target))
}

func TestWriter_Write_TwoBlocks_ReplaceOne_LeavesOther(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	original := "<!-- archy:start id=alpha -->\nA1\n<!-- archy:end -->\n\n<!-- archy:start id=beta -->\nB1\n<!-- archy:end -->\n"
	writeFile(t, target, original)

	_, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "alpha",
		Content:  "A2",
	})
	require.NoError(t, err)

	expected := "<!-- archy:start id=alpha -->\nA2\n<!-- archy:end -->\n\n<!-- archy:start id=beta -->\nB1\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, target))
}

func TestWriter_Write_DuplicateMarker_ErrorsAndDoesNotWrite(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	original := "<!-- archy:start id=foo -->\none\n<!-- archy:end -->\n<!-- archy:start id=foo -->\ntwo\n<!-- archy:end -->\n"
	writeFile(t, target, original)

	_, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "ignored",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicateMarker), "expected ErrDuplicateMarker, got %v", err)
	assert.Equal(t, original, readFile(t, target), "file should be unchanged")
}

func TestWriter_Write_PathOutsideVault_Absolute(t *testing.T) {
	w, _ := newTestWriter(t)
	other := t.TempDir()
	target := filepath.Join(other, "leaked.md")

	_, err := w.Write(context.Background(), Note{
		Path:     target,
		Mode:     ModeMarkerBlock,
		MarkerID: "x",
		Content:  "no",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathEscape))
	_, statErr := os.Stat(target)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "no file should have been created")
}

func TestWriter_Write_PathDotDotInsideVault_Allowed(t *testing.T) {
	w, dir := newTestWriter(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	_, err := w.Write(context.Background(), Note{
		Path:     "sub/../note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "hi",
	})
	require.NoError(t, err)
	expected := "<!-- archy:start id=foo -->\nhi\n<!-- archy:end -->\n"
	assert.Equal(t, expected, readFile(t, filepath.Join(dir, "note.md")))
}

func TestWriter_Write_PathDotDotEscapesVault(t *testing.T) {
	w, _ := newTestWriter(t)
	_, err := w.Write(context.Background(), Note{
		Path:     "../escape.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "no",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathEscape))
}

func TestWriter_Write_ParentDirectoryCreated(t *testing.T) {
	w, dir := newTestWriter(t)
	_, err := w.Write(context.Background(), Note{
		Path:     "deeply/nested/folders/note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "hi",
	})
	require.NoError(t, err)

	target := filepath.Join(dir, "deeply", "nested", "folders", "note.md")
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.False(t, info.IsDir())

	parent, err := os.Stat(filepath.Join(dir, "deeply", "nested", "folders"))
	require.NoError(t, err)
	assert.True(t, parent.IsDir())
	// 0755 (mode bits other than directory bit).
	assert.Equal(t, os.FileMode(0o755), parent.Mode().Perm())
}

func TestWriter_Write_Overwrite(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	writeFile(t, target, "old\nstuff\n")

	res, err := w.Write(context.Background(), Note{
		Path:    "note.md",
		Mode:    ModeOverwrite,
		Content: "totally new content",
	})
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.Equal(t, "totally new content\n", readFile(t, target))
}

func TestWriter_Write_Overwrite_CreatesNewFile(t *testing.T) {
	w, dir := newTestWriter(t)
	res, err := w.Write(context.Background(), Note{
		Path:    "fresh.md",
		Mode:    ModeOverwrite,
		Content: "hello",
	})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, "hello\n", readFile(t, filepath.Join(dir, "fresh.md")))
}

func TestWriter_Write_Append_NewFile(t *testing.T) {
	w, dir := newTestWriter(t)
	res, err := w.Write(context.Background(), Note{
		Path:    "log.md",
		Mode:    ModeAppend,
		Content: "first entry\n",
	})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, "first entry\n", readFile(t, filepath.Join(dir, "log.md")))
}

func TestWriter_Write_Append_ExistingWithTrailingNewline(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "log.md")
	writeFile(t, target, "first\n")

	_, err := w.Write(context.Background(), Note{
		Path:    "log.md",
		Mode:    ModeAppend,
		Content: "second\n",
	})
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", readFile(t, target))
}

func TestWriter_Write_Append_ExistingWithoutTrailingNewline(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "log.md")
	writeFile(t, target, "first")

	_, err := w.Write(context.Background(), Note{
		Path:    "log.md",
		Mode:    ModeAppend,
		Content: "second\n",
	})
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", readFile(t, target))
}

func TestWriter_Write_UnchangedContent_NoOp(t *testing.T) {
	w, dir := newTestWriter(t)
	note := Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "daily-brief",
		Content:  "same content",
	}
	_, err := w.Write(context.Background(), note)
	require.NoError(t, err)
	target := filepath.Join(dir, "note.md")
	infoBefore, err := os.Stat(target)
	require.NoError(t, err)

	res, err := w.Write(context.Background(), note)
	require.NoError(t, err)
	assert.Equal(t, 0, res.BytesWritten)
	assert.False(t, res.BlockAdded)
	assert.False(t, res.BlockUpdated)
	assert.False(t, res.Created)

	infoAfter, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime(), "mtime should not advance")
}

func TestWriter_Write_ConcurrentDifferentFiles(t *testing.T) {
	w, dir := newTestWriter(t)

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := w.Write(context.Background(), Note{
				Path:     fmt.Sprintf("note-%d.md", i),
				Mode:     ModeMarkerBlock,
				MarkerID: "daily-brief",
				Content:  fmt.Sprintf("entry %d", i),
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "goroutine %d", i)
		expected := fmt.Sprintf("<!-- archy:start id=daily-brief -->\nentry %d\n<!-- archy:end -->\n", i)
		assert.Equal(t, expected, readFile(t, filepath.Join(dir, fmt.Sprintf("note-%d.md", i))))
	}
}

func TestWriter_Write_InvalidMarkerID(t *testing.T) {
	w, _ := newTestWriter(t)
	note := Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "bad id with space",
		Content:  "hi",
	}
	require.True(t, errors.Is(note.Validate(), ErrInvalidMarkerID))

	_, err := w.Write(context.Background(), note)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidMarkerID))
}

func TestWriter_Write_EmptyPath(t *testing.T) {
	w, _ := newTestWriter(t)
	_, err := w.Write(context.Background(), Note{
		Path:     "",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "x",
	})
	require.Error(t, err)
}

func TestNew_NonexistentVaultRoot(t *testing.T) {
	_, err := New("/this/does/not/exist/anywhere")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVaultRootInvalid))
}

func TestNew_VaultRootIsFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "not-a-dir")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o644))

	_, err := New(target)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVaultRootInvalid))
}

func TestNew_RelativePath(t *testing.T) {
	_, err := New("relative/path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVaultRootInvalid))
}

func TestWriter_Frontmatter_DeterministicOrder(t *testing.T) {
	// Run twice; expect identical output despite map iteration order.
	for i := 0; i < 5; i++ {
		w, dir := newTestWriter(t)
		_, err := w.Write(context.Background(), Note{
			Path:     "note.md",
			Mode:     ModeMarkerBlock,
			MarkerID: "x",
			Content:  "c",
			Frontmatter: map[string]any{
				"alpha":   "1",
				"bravo":   "2",
				"charlie": "3",
				"delta":   "4",
			},
		})
		require.NoError(t, err)
		got := readFile(t, filepath.Join(dir, "note.md"))
		expected := "---\nalpha: \"1\"\nbravo: \"2\"\ncharlie: \"3\"\ndelta: \"4\"\n---\n\n<!-- archy:start id=x -->\nc\n<!-- archy:end -->\n"
		assert.Equal(t, expected, got, "iteration %d", i)
	}
}

func TestWriter_Frontmatter_IgnoredOnExistingFile(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	writeFile(t, target, "# pre-existing\n")

	_, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "x",
		Content:  "body",
		Frontmatter: map[string]any{
			"title": "should not appear",
		},
	})
	require.NoError(t, err)

	got := readFile(t, target)
	assert.NotContains(t, got, "title: should not appear")
	assert.Contains(t, got, "# pre-existing")
}

func TestWriter_Write_CRLFExistingFile_PreservesNonBlockBytes(t *testing.T) {
	w, dir := newTestWriter(t)
	target := filepath.Join(dir, "note.md")
	original := "line one\r\nline two\r\n\r\n<!-- archy:start id=foo -->\r\nold\r\n<!-- archy:end -->\r\nfooter\r\n"
	writeFile(t, target, original)

	_, err := w.Write(context.Background(), Note{
		Path:     "note.md",
		Mode:     ModeMarkerBlock,
		MarkerID: "foo",
		Content:  "new",
	})
	require.NoError(t, err)

	got := readFile(t, target)
	// Non-block content (with CRLF) is preserved byte-for-byte except the
	// trailing newline. The block itself is rewritten with LF endings.
	assert.Contains(t, got, "line one\r\nline two\r\n")
	assert.Contains(t, got, "<!-- archy:start id=foo -->\nnew\n<!-- archy:end -->")
	assert.True(t, len(got) > 0 && got[len(got)-1] == '\n')
}
