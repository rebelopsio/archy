package write

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMarkerID_Accept(t *testing.T) {
	cases := []string{
		"foo",
		"daily-brief",
		"a",
		"daily-brief-2026",
		"abc123",
		"a1",
		"1a",
	}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			assert.NoError(t, ValidateMarkerID(id))
		})
	}
}

func TestValidateMarkerID_Reject(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"leading-space", " foo"},
		{"trailing-space", "foo "},
		{"interior-space", "foo bar"},
		{"leading-hyphen", "-foo"},
		{"trailing-hyphen", "foo-"},
		{"newline", "foo\nbar"},
		{"double-quote", "foo\"bar"},
		{"single-quote", "foo'bar"},
		{"slash", "foo/bar"},
		{"dot", "foo.bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMarkerID(tc.id)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidMarkerID), "expected ErrInvalidMarkerID, got %v", err)
		})
	}
}

func TestSerializeMarkerBlock(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		content string
		want    string
	}{
		{
			name:    "no-trailing-newline",
			id:      "foo",
			content: "hello",
			want:    "<!-- archy:start id=foo -->\nhello\n<!-- archy:end -->",
		},
		{
			name:    "single-trailing-newline",
			id:      "foo",
			content: "hello\n",
			want:    "<!-- archy:start id=foo -->\nhello\n<!-- archy:end -->",
		},
		{
			name:    "multiple-trailing-newlines",
			id:      "foo",
			content: "hello\n\n\n",
			want:    "<!-- archy:start id=foo -->\nhello\n<!-- archy:end -->",
		},
		{
			name:    "empty-content",
			id:      "foo",
			content: "",
			want:    "<!-- archy:start id=foo -->\n\n<!-- archy:end -->",
		},
		{
			name:    "hyphenated-id",
			id:      "daily-brief-2026",
			content: "line one\nline two",
			want:    "<!-- archy:start id=daily-brief-2026 -->\nline one\nline two\n<!-- archy:end -->",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := serializeMarkerBlock(tc.id, tc.content)
			assert.Equal(t, tc.want, got)
		})
	}
}
