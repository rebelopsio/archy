package linear

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

// TestPriorityFromLinear_Mapping is the load-bearing test for the
// priority conversion table. Linear's numeric scheme is inverted from
// intuition (1 = highest, 4 = lowest); this test asserts each value
// explicitly so a future drive-by refactor can't silently invert.
func TestPriorityFromLinear_Mapping(t *testing.T) {
	cases := []struct {
		linearValue int
		linearName  string
		want        domain.Priority
	}{
		{0, "No priority", domain.PriorityUnknown},
		{1, "Urgent", domain.PriorityUrgent},
		{2, "High", domain.PriorityHigh},
		{3, "Medium", domain.PriorityMedium},
		{4, "Low", domain.PriorityLow},
	}
	for _, tc := range cases {
		t.Run(tc.linearName, func(t *testing.T) {
			got := priorityFromLinear(&linearPriority{Value: tc.linearValue, Name: tc.linearName})
			assert.Equalf(t, tc.want, got,
				"Linear value %d (%q) should map to %s, got %s",
				tc.linearValue, tc.linearName, tc.want, got)
		})
	}
}

func TestPriorityFromLinear_NilReturnsUnknown(t *testing.T) {
	assert.Equal(t, domain.PriorityUnknown, priorityFromLinear(nil))
}

func TestPriorityFromLinear_OutOfRangeReturnsUnknown(t *testing.T) {
	cases := []int{-1, 5, 99, 1000}
	for _, v := range cases {
		got := priorityFromLinear(&linearPriority{Value: v})
		assert.Equal(t, domain.PriorityUnknown, got, "value %d", v)
	}
}

// TestPriorityFromLinear_IgnoresName confirms the conversion uses Value
// only — Name is informational.
func TestPriorityFromLinear_IgnoresName(t *testing.T) {
	// Pretend Linear returns a misleading name; conversion follows Value.
	got := priorityFromLinear(&linearPriority{Value: 1, Name: "totally not urgent"})
	assert.Equal(t, domain.PriorityUrgent, got)
}

func TestStateFromLinear_StandardSet(t *testing.T) {
	cases := map[string]domain.IssueState{
		"backlog":   domain.IssueStateBacklog,
		"unstarted": domain.IssueStateTodo,
		"started":   domain.IssueStateInProgress,
		"completed": domain.IssueStateDone,
		"canceled":  domain.IssueStateCanceled,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, stateFromLinear(in))
		})
	}
}

func TestStateFromLinear_UnknownReturnsUnknown(t *testing.T) {
	cases := []string{"", "triaged", "in_progress", "DONE", "started ", "backlog\n", "bogus"}
	for _, in := range cases {
		got := stateFromLinear(in)
		// "DONE" and "started " (trailing space) are normalized via lower+trim;
		// they should match. Others should be unknown.
		switch strings.ToLower(strings.TrimSpace(in)) {
		case "started", "completed", "backlog", "unstarted", "canceled":
			assert.NotEqual(t, domain.IssueStateUnknown, got, "input %q normalized to a known state", in)
		default:
			assert.Equalf(t, domain.IssueStateUnknown, got, "input %q should be unknown", in)
		}
	}
}

func TestStateFromLinear_BritishCancelledAccepted(t *testing.T) {
	// Linear uses American "canceled" but the British "cancelled" shows
	// up in plenty of UIs; accept both as a kindness.
	assert.Equal(t, domain.IssueStateCanceled, stateFromLinear("cancelled"))
}

func TestIssueFromLinear_PopulatesProvider(t *testing.T) {
	got := issueFromLinear(linearIssue{ID: "ENG-1", URL: "https://linear.app/x/issue/ENG-1"})
	assert.Equal(t, "linear", got.Ref.Provider)
	assert.Equal(t, "ENG-1", got.Ref.ID)
	assert.Equal(t, "https://linear.app/x/issue/ENG-1", got.Ref.URL)
}

func TestIssueFromLinear_Roundtrip(t *testing.T) {
	got := issueFromLinear(linearIssue{
		ID:          "SOC-26",
		URL:         "https://linear.app/x/issue/SOC-26",
		Title:       "Fix the thing",
		Description: "It is broken.",
		Priority:    &linearPriority{Value: 1, Name: "Urgent"},
		StatusType:  "started",
		Assignee:    "Ada Lovelace",
		AssigneeID:  "861548d1-a0c1-4091-af9d-3f727b420fca",
		DueDate:     "2026-05-10",
		CreatedAt:   "2026-05-01T09:00:00Z",
		UpdatedAt:   "2026-05-06T12:00:00.123456Z",
	})
	assert.Equal(t, "Fix the thing", got.Title)
	assert.Equal(t, "It is broken.", got.Description)
	assert.Equal(t, domain.PriorityUrgent, got.Priority)
	assert.Equal(t, domain.IssueStateInProgress, got.State)
	require.NotNil(t, got.Assignee)
	assert.Equal(t, "Ada Lovelace", got.Assignee.Name)
	// Email and Username are no longer populated — the new wire shape
	// gives us only the display name. AssigneeID is captured on the
	// wire type for future consumers but has no home on domain.Person.
	assert.Empty(t, got.Assignee.Email)
	assert.Empty(t, got.Assignee.Username)
	assert.Equal(t, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), got.DueAt)
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestIssueFromLinear_AssigneeFromStringAndID(t *testing.T) {
	li := linearIssue{
		ID:         "ENG-1",
		Title:      "test",
		Assignee:   "Stephen Morgan",
		AssigneeID: "861548d1-a0c1-4091-af9d-3f727b420fca",
		StatusType: "started",
	}
	got := issueFromLinear(li)
	require.NotNil(t, got.Assignee)
	assert.Equal(t, "Stephen Morgan", got.Assignee.Name)
	// domain.Person has no ID field; the UUID lives on linearIssue.
	assert.Equal(t, "861548d1-a0c1-4091-af9d-3f727b420fca", li.AssigneeID)
}

func TestIssueFromLinear_UnassignedLeavesAssigneeZero(t *testing.T) {
	li := linearIssue{
		ID:         "ENG-1",
		Title:      "test",
		StatusType: "backlog",
	}
	got := issueFromLinear(li)
	assert.Nil(t, got.Assignee)
}

func TestPersonFromLinear_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, personFromLinear(""))
}

func TestPersonFromLinear_PopulatesName(t *testing.T) {
	got := personFromLinear("Ada Lovelace")
	require.NotNil(t, got)
	assert.Equal(t, "Ada Lovelace", got.Name)
	assert.Empty(t, got.Email)
	assert.Empty(t, got.Username)
}

func TestParseDueDate(t *testing.T) {
	t.Run("yyyy-mm-dd", func(t *testing.T) {
		got := parseDueDate("2026-05-10")
		assert.Equal(t, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), got)
	})
	t.Run("rfc3339-also-accepted", func(t *testing.T) {
		got := parseDueDate("2026-05-10T15:30:00Z")
		assert.False(t, got.IsZero())
	})
	t.Run("empty", func(t *testing.T) {
		assert.True(t, parseDueDate("").IsZero())
	})
	t.Run("garbage-returns-zero", func(t *testing.T) {
		assert.True(t, parseDueDate("not a date").IsZero())
	})
}
