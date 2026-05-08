package domain

import "time"

// CalendarEvent is a calendar item: a Google Calendar event, an Outlook
// event, etc.
type CalendarEvent struct {
	// Ref is the provider-agnostic identifier.
	Ref ExternalRef

	// Title is the event's headline.
	Title string
	// Description is the event body, if any.
	Description string

	// Location is the meeting location, room, or video link.
	Location string

	// StartAt is the event start time.
	StartAt time.Time
	// EndAt is the event end time.
	EndAt time.Time

	// AllDay is true for all-day events. When true, StartAt is the start
	// of the day in the event's local time and EndAt is the end of the
	// day; the times themselves are not significant.
	AllDay bool

	// Attendees lists the people invited. The organizer is included.
	Attendees []Person
	// Organizer is the event creator. May be nil if unknown.
	Organizer *Person

	// CalendarName is the human-readable calendar this event came from.
	CalendarName string
}

// Duration returns EndAt minus StartAt. May be negative if dates are
// inverted; that is the caller's problem to detect.
func (e CalendarEvent) Duration() time.Duration {
	return e.EndAt.Sub(e.StartAt)
}

// IsHappeningAt reports whether the event is currently occurring at t,
// using a half-open interval: StartAt <= t < EndAt. A meeting that ends
// exactly at t is not "happening" at t. For all-day events the same
// logic applies — StartAt and EndAt describe the date range.
func (e CalendarEvent) IsHappeningAt(t time.Time) bool {
	if e.StartAt.After(t) {
		return false
	}
	if !e.EndAt.After(t) {
		return false
	}
	return true
}

// HasExternalAttendees reports whether any attendee has a non-empty
// email that is not one of the operator's registered emails (per
// Identity.MatchesEmail, case-insensitive). Empty attendee emails are
// skipped. Returns false when id has no emails configured — without
// any "me" emails we cannot label anyone as "external".
func (e CalendarEvent) HasExternalAttendees(id Identity) bool {
	if len(id.Emails) == 0 {
		return false
	}
	for _, a := range e.Attendees {
		if a.Email == "" {
			continue
		}
		if !id.MatchesEmail(a.Email) {
			return true
		}
	}
	return false
}
