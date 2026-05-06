package write

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_NoFrontmatterNoMarkers(t *testing.T) {
	in := "# heading\n\nsome text\nmore text\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	assert.Empty(t, pf.blocks)
}

func TestParse_FrontmatterNoMarkers(t *testing.T) {
	in := "---\ntitle: hello\ndate: 2026-05-06\n---\n\nbody text here.\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	assert.Empty(t, pf.blocks)
}

func TestParse_SingleMarkerBlock(t *testing.T) {
	in := "before\n<!-- archy:start id=foo -->\nhello\n<!-- archy:end -->\nafter\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.Len(t, pf.blocks, 1)
	b := pf.blocks[0]
	assert.Equal(t, "foo", b.id)
	assert.Equal(t, "hello\n", string(pf.raw[b.contentStart:b.contentEnd]))
	// The block range covers the start line, content, and end line including trailing newline.
	assert.Equal(t,
		"<!-- archy:start id=foo -->\nhello\n<!-- archy:end -->\n",
		string(pf.raw[b.start:b.end]),
	)
}

func TestParse_TwoMarkerBlocks_DifferentIDs(t *testing.T) {
	in := "<!-- archy:start id=alpha -->\nA\n<!-- archy:end -->\n\n<!-- archy:start id=beta -->\nB\n<!-- archy:end -->\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.Len(t, pf.blocks, 2)
	assert.Equal(t, "alpha", pf.blocks[0].id)
	assert.Equal(t, "beta", pf.blocks[1].id)
	assert.Equal(t, "A\n", string(pf.raw[pf.blocks[0].contentStart:pf.blocks[0].contentEnd]))
	assert.Equal(t, "B\n", string(pf.raw[pf.blocks[1].contentStart:pf.blocks[1].contentEnd]))
}

func TestParse_DuplicateMarkerID(t *testing.T) {
	in := "<!-- archy:start id=foo -->\none\n<!-- archy:end -->\n<!-- archy:start id=foo -->\ntwo\n<!-- archy:end -->\n"
	_, err := parse([]byte(in))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicateMarker), "expected ErrDuplicateMarker, got %v", err)
	assert.Contains(t, err.Error(), `"foo"`)
}

func TestParse_BacktickFencedMarkersIgnored(t *testing.T) {
	in := "```markdown\n<!-- archy:start id=foo -->\nfake\n<!-- archy:end -->\n```\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	assert.Empty(t, pf.blocks)
}

func TestParse_TildeFencedMarkersIgnored(t *testing.T) {
	in := "~~~\n<!-- archy:start id=foo -->\nfake\n<!-- archy:end -->\n~~~\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	assert.Empty(t, pf.blocks)
}

func TestParse_CRLFLineEndings(t *testing.T) {
	in := "before\r\n<!-- archy:start id=foo -->\r\nhello\r\n<!-- archy:end -->\r\nafter\r\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.Len(t, pf.blocks, 1)
	assert.Equal(t, "foo", pf.blocks[0].id)
}

func TestParse_StartWithoutEnd(t *testing.T) {
	in := "<!-- archy:start id=foo -->\nbody but no end\n"
	_, err := parse([]byte(in))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnclosedMarker), "expected ErrUnclosedMarker, got %v", err)
}

func TestParse_EndWithoutStart(t *testing.T) {
	in := "some text\n<!-- archy:end -->\nmore text\n"
	_, err := parse([]byte(in))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnclosedMarker), "expected ErrUnclosedMarker, got %v", err)
}

func TestParse_NonCanonicalCommentsIgnored(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"no-spaces", "<!--archy:start id=foo-->"},
		{"extra-internal-space", "<!-- archy:start  id=foo -->"},
		{"tab-internal", "<!-- archy:start\tid=foo -->"},
		{"missing-id-prefix", "<!-- archy:start foo -->"},
		{"trailing-content", "<!-- archy:start id=foo --> trailing"},
		{"end-with-attrs", "<!-- archy:end id=foo -->"},
		{"invalid-id-trailing-hyphen", "<!-- archy:start id=foo- -->"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.line + "\nbody\n"
			pf, err := parse([]byte(in))
			require.NoError(t, err)
			assert.Empty(t, pf.blocks, "line should not have been parsed as a marker")
		})
	}
}

func TestParse_HyphenatedID(t *testing.T) {
	in := "<!-- archy:start id=daily-brief-2026 -->\nhi\n<!-- archy:end -->\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.Len(t, pf.blocks, 1)
	assert.Equal(t, "daily-brief-2026", pf.blocks[0].id)
}

func TestParse_FindBlock(t *testing.T) {
	in := "<!-- archy:start id=alpha -->\nA\n<!-- archy:end -->\n<!-- archy:start id=beta -->\nB\n<!-- archy:end -->\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.NotNil(t, pf.findBlock("alpha"))
	require.NotNil(t, pf.findBlock("beta"))
	assert.Nil(t, pf.findBlock("gamma"))
}

func TestParse_FenceClosingRequiresMatchingChar(t *testing.T) {
	// A backtick fence is not closed by a tilde line, so the marker stays inside the fence.
	in := "```\n<!-- archy:start id=foo -->\n~~~\nfake\n<!-- archy:end -->\n```\n"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	assert.Empty(t, pf.blocks)
}

func TestParse_NoTrailingNewline(t *testing.T) {
	in := "<!-- archy:start id=foo -->\nhi\n<!-- archy:end -->"
	pf, err := parse([]byte(in))
	require.NoError(t, err)
	require.Len(t, pf.blocks, 1)
	b := pf.blocks[0]
	assert.Equal(t, len(in), b.end, "end should reach end-of-input when no trailing newline")
	assert.Equal(t, "hi\n", string(pf.raw[b.contentStart:b.contentEnd]))
}

// Sanity: parse should not touch raw bytes — it just tokenizes.
func TestParse_DoesNotMutateRaw(t *testing.T) {
	in := []byte("<!-- archy:start id=foo -->\nhi\n<!-- archy:end -->\n")
	original := make([]byte, len(in))
	copy(original, in)
	_, err := parse(in)
	require.NoError(t, err)
	assert.Equal(t, original, in)
	// Sanity: no surprising rewriting is happening.
	assert.True(t, strings.HasPrefix(string(in), "<!-- archy:start"))
}
