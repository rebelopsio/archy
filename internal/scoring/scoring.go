package scoring

import "time"

// Weights configures how much each signal contributes when triggered.
// All weights must be non-negative; zero disables the signal but still
// records it in [domain.PriorityScore.Signals] so --explain can show
// "checked but disabled."
type Weights struct {
	// MeetingSoon fires for calendar events starting within MeetingSoonWindow.
	MeetingSoon int
	// UrgentIssue fires for issues at PriorityHigh or PriorityUrgent.
	UrgentIssue int
	// ReviewRequested fires for PRs awaiting the user's review.
	ReviewRequested int
	// OverdueIssue fires for issues past DueAt and not yet terminal.
	OverdueIssue int
	// BlockedOnUser fires for open PRs awaiting the user with CI passing
	// or unknown.
	BlockedOnUser int
	// StaleItem fires for issues or PRs older than StaleAfter with no update.
	StaleItem int
	// CIFailing fires for open PRs with CIPassing == false.
	CIFailing int
	// ExternalAttendees fires for events with non-user-domain attendees.
	ExternalAttendees int
	// KeyStakeholder fires for events organized by anyone in
	// Context.KeyStakeholders.
	KeyStakeholder int
}

// Thresholds tunes signal-firing windows. The zero value is replaced
// field-by-field with [DefaultThresholds] at evaluation time, so callers
// can override one threshold without losing the other.
type Thresholds struct {
	// MeetingSoonWindow is how soon "meeting soon" fires before StartAt.
	// Default 30 minutes.
	MeetingSoonWindow time.Duration
	// StaleAfter is how long without an UpdatedAt change before "stale"
	// fires. Default 14 days.
	StaleAfter time.Duration
}

// DefaultThresholds returns archy's default thresholds: 30-minute
// meeting-soon window and a 14-day stale window.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MeetingSoonWindow: 30 * time.Minute,
		StaleAfter:        14 * 24 * time.Hour,
	}
}

// Context bundles the data signals need that isn't on the items themselves.
// Construct one per scoring run.
type Context struct {
	// Now is the reference time for time-dependent signals. Signals never
	// call time.Now() — they read from this field.
	Now time.Time
	// UserEmail is the operating user's primary email, used by signals
	// that distinguish the user from other people.
	UserEmail string
	// UserUsername is the operating user's provider handle.
	UserUsername string
	// KeyStakeholders is a set of usernames or emails treated as
	// important for the key_stakeholder signal. Empty disables the signal.
	KeyStakeholders []string
	// Weights and Thresholds may be the zero value; in that case the
	// per-signal default is used. An all-zero Weights produces all-zero
	// scores — callers should usually populate Weights from config.
	Weights    Weights
	Thresholds Thresholds
}
