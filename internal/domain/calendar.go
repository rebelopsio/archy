package domain

import (
	"strings"
	"time"
)

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
// email whose domain (the part after "@") differs from userEmail's
// domain (case-insensitive). Empty attendee emails are skipped. If
// userEmail has no "@", returns false — we can't determine who is
// "external" without knowing the user's domain.
func (e CalendarEvent) HasExternalAttendees(userEmail string) bool {
	userDomain, ok := emailDomain(userEmail)
	if !ok {
		return false
	}
	for _, a := range e.Attendees {
		if a.Email == "" {
			continue
		}
		d, ok := emailDomain(a.Email)
		if !ok {
			continue
		}
		if d != userDomain {
			return true
		}
	}
	return false
}

// emailDomain returns the lowercase domain portion of email (after the
// last "@"). The second return is false if no "@" is present or the
// domain is empty.
func emailDomain(email string) (string, bool) {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return "", false
	}
	return strings.ToLower(email[at+1:]), true
}
