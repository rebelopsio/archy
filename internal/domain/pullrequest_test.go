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

	t.Run("linear-handle-match", func(t *testing.T) {
		me := MakeIdentity([]string{"u@e.com"}, "ada", "")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("github-handle-match", func(t *testing.T) {
		me := MakeIdentity([]string{"u@e.com"}, "", "ada")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("handle-match-case-insensitive", func(t *testing.T) {
		me := MakeIdentity([]string{"u@e.com"}, "ADA", "")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("email-match-when-reviewer-username-missing", func(t *testing.T) {
		// Grace has no Username — fall back to email set membership.
		me := MakeIdentity([]string{"grace@example.com"}, "", "")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("email-match-case-insensitive", func(t *testing.T) {
		me := MakeIdentity([]string{"GRACE@EXAMPLE.COM"}, "", "")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("alt-email-recognized", func(t *testing.T) {
		me := MakeIdentity([]string{"primary@x.com", "grace@example.com"}, "", "")
		assert.True(t, pr.IsBlockedOnUser(me))
	})

	t.Run("no-handles-and-no-email-match", func(t *testing.T) {
		me := MakeIdentity([]string{"alan@example.com"}, "alan", "alan")
		assert.False(t, pr.IsBlockedOnUser(me))
	})

	t.Run("reviewer-with-username-does-not-fall-through-to-email", func(t *testing.T) {
		// ada has a username; even if we know ada's email, an Identity
		// without a matching handle should NOT match this reviewer via
		// email — once a reviewer has a Username, that's what we match on.
		me := MakeIdentity([]string{"ada@example.com"}, "alan", "alan")
		assert.False(t, pr.IsBlockedOnUser(me))
	})

	t.Run("empty-reviewers", func(t *testing.T) {
		emptyPR := PullRequest{}
		me := MakeIdentity([]string{"u@e.com"}, "ada", "")
		assert.False(t, emptyPR.IsBlockedOnUser(me))
	})
}
