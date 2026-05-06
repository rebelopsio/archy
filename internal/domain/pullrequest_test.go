package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPullRequestState_String(t *testing.T) {
	cases := []struct {
		name string
		s    PullRequestState
		want string
	}{
		{"unknown", PullRequestStateUnknown, "unknown"},
		{"draft", PullRequestStateDraft, "draft"},
		{"open", PullRequestStateOpen, "open"},
		{"merged", PullRequestStateMerged, "merged"},
		{"closed", PullRequestStateClosed, "closed"},
		{"out-of-range", PullRequestState(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.s.String())
		})
	}
}

func TestPullRequestState_IsTerminal(t *testing.T) {
	cases := []struct {
		name string
		s    PullRequestState
		want bool
	}{
		{"unknown", PullRequestStateUnknown, false},
		{"draft", PullRequestStateDraft, false},
		{"open", PullRequestStateOpen, false},
		{"merged", PullRequestStateMerged, true},
		{"closed", PullRequestStateClosed, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.s.IsTerminal())
		})
	}
}

func TestPullRequest_IsBlockedOnUser(t *testing.T) {
	pr := PullRequest{
		RequestedReviewers: []Person{
			{Name: "Ada Lovelace", Username: "ada", Email: "ada@example.com"},
			{Name: "Grace Hopper", Email: "grace@example.com"}, // no username
		},
	}

	t.Run("username-match", func(t *testing.T) {
		assert.True(t, pr.IsBlockedOnUser(Person{Username: "ada"}))
	})

	t.Run("username-case-sensitive", func(t *testing.T) {
		assert.False(t, pr.IsBlockedOnUser(Person{Username: "Ada"}))
	})

	t.Run("email-match-when-username-missing", func(t *testing.T) {
		// Person has no Username; reviewer Grace has no Username — fall back to email.
		assert.True(t, pr.IsBlockedOnUser(Person{Email: "grace@example.com"}))
	})

	t.Run("email-match-case-insensitive", func(t *testing.T) {
		assert.True(t, pr.IsBlockedOnUser(Person{Email: "GRACE@EXAMPLE.COM"}))
	})

	t.Run("name-not-used-for-match", func(t *testing.T) {
		// Two people named Ada, but no username/email overlap.
		other := Person{Name: "Ada Lovelace", Username: "different-ada"}
		assert.False(t, pr.IsBlockedOnUser(other))
	})

	t.Run("no-match", func(t *testing.T) {
		assert.False(t, pr.IsBlockedOnUser(Person{Username: "alan"}))
	})

	t.Run("empty-reviewers", func(t *testing.T) {
		emptyPR := PullRequest{}
		assert.False(t, emptyPR.IsBlockedOnUser(Person{Username: "ada"}))
	})

	t.Run("username-mismatch-does-not-fall-through-to-email", func(t *testing.T) {
		// ada has username; querying with a different username + ada's
		// email should NOT match: when both have usernames, only username
		// counts.
		assert.False(t, pr.IsBlockedOnUser(Person{Username: "alan", Email: "ada@example.com"}))
	})
}
