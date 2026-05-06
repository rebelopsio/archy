package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriority_String(t *testing.T) {
	cases := []struct {
		name string
		p    Priority
		want string
	}{
		{"unknown-zero-value", PriorityUnknown, "unknown"},
		{"low", PriorityLow, "low"},
		{"medium", PriorityMedium, "medium"},
		{"high", PriorityHigh, "high"},
		{"urgent", PriorityUrgent, "urgent"},
		{"out-of-range", Priority(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.p.String())
		})
	}
}
