package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressKind_String(t *testing.T) {
	cases := []struct {
		k    ProgressKind
		want string
	}{
		{ProgressUnknown, "unknown"},
		{ProgressStart, "start"},
		{ProgressToolCall, "tool_call"},
		{ProgressTextChunk, "text_chunk"},
		{ProgressTurnComplete, "turn_complete"},
		{ProgressEnd, "end"},
		{ProgressKind(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.k.String())
		})
	}
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "abcde", truncate("abcde", 5))
	assert.Equal(t, "abcde...", truncate("abcdefgh", 5))
}
