package domain

import (
	"strings"
	"time"
)

// PullRequestState is the canonical lifecycle state of a pull request.
type PullRequestState int

// PullRequestState values from creation through completion.
const (
	// PullRequestStateUnknown is the zero value.
	PullRequestStateUnknown PullRequestState = iota
	// PullRequestStateDraft is opened in draft status.
	PullRequestStateDraft
	// PullRequestStateOpen is open for review and merge.
	PullRequestStateOpen
	// PullRequestStateMerged was merged into the target branch.
	PullRequestStateMerged
	// PullRequestStateClosed was closed without merging.
	PullRequestStateClosed
)

// String returns the state's lowercase name. Out-of-range values return
// "unknown".
func (s PullRequestState) String() string {
	switch s {
	case PullRequestStateDraft:
		return "draft"
	case PullRequestStateOpen:
		return "open"
	case PullRequestStateMerged:
		return "merged"
	case PullRequestStateClosed:
		return "closed"
	case PullRequestStateUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// IsTerminal reports whether the state means the PR is no longer active
// (Merged or Closed).
func (s PullRequestState) IsTerminal() bool {
	return s == PullRequestStateMerged || s == PullRequestStateClosed
}

// PullRequest is a code review item: a GitHub PR, a GitLab MR, etc.
type PullRequest struct {
	// Ref is the provider-agnostic identifier.
	Ref ExternalRef

	// Title is the PR's headline.
	Title string
	// Description is the PR body, if any.
	Description string

	// State is the canonical lifecycle state.
	State PullRequestState

	// Author is the person who opened the PR. Nil if unknown.
	Author *Person
	// Assignees are people assigned to the PR.
	Assignees []Person

	// RequestedReviewers are people whose review has been requested but
	// not yet provided. As reviewers complete their reviews they leave
	// this list.
	RequestedReviewers []Person

	// Repository is the human-readable repo name, e.g. "owner/repo".
	Repository string
	// Branch is the source branch name (the head, not the base).
	Branch string

	// Labels applied to the PR.
	Labels []string

	// CreatedAt is the PR creation time.
	CreatedAt time.Time
	// UpdatedAt is the last modification time.
	UpdatedAt time.Time
	// MergedAt is the merge time, zero if not merged.
	MergedAt time.Time

	// CIPassing is true if all required checks are green, false if any
	// are failing or pending. Nil means CI status is unknown.
	CIPassing *bool
}

// IsBlockedOnUser reports whether the given person appears in
// RequestedReviewers — i.e., the PR is waiting on them.
//
// Matching strategy, per requested reviewer:
//   - If both person and reviewer have a non-empty Username, match by
//     Username (case-sensitive).
//   - Otherwise, if both have a non-empty Email, match by Email
//     (case-insensitive).
//   - Otherwise that reviewer does not match.
//
// Name is never used for matching — two people can share a name.
func (p PullRequest) IsBlockedOnUser(person Person) bool {
	for _, r := range p.RequestedReviewers {
		if person.Username != "" && r.Username != "" {
			if person.Username == r.Username {
				return true
			}
			continue
		}
		if person.Email != "" && r.Email != "" {
			if strings.EqualFold(person.Email, r.Email) {
				return true
			}
		}
	}
	return false
}
