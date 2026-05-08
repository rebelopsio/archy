package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalendarEvent_Duration(t *testing.T) {
	start := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
	t.Run("normal", func(t *testing.T) {
		e := CalendarEvent{StartAt: start, EndAt: start.Add(45 * time.Minute)}
		assert.Equal(t, 45*time.Minute, e.Duration())
	})
	t.Run("zero-when-equal", func(t *testing.T) {
		e := CalendarEvent{StartAt: start, EndAt: start}
		assert.Equal(t, time.Duration(0), e.Duration())
	})
	t.Run("negative-when-inverted", func(t *testing.T) {
		e := CalendarEvent{StartAt: start, EndAt: start.Add(-30 * time.Minute)}
		assert.Equal(t, -30*time.Minute, e.Duration())
	})
}

func TestCalendarEvent_IsHappeningAt(t *testing.T) {
	start := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	e := CalendarEvent{StartAt: start, EndAt: end}

	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"strictly-inside", start.Add(30 * time.Minute), true},
		{"equal-to-start-closed", start, true},
		{"equal-to-end-open", end, false},
		{"before-start", start.Add(-time.Minute), false},
		{"after-end", end.Add(time.Minute), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, e.IsHappeningAt(tc.t))
		})
	}
}

func TestCalendarEvent_HasExternalAttendees(t *testing.T) {
	me := MakeIdentity([]string{"user@example.com"}, "", "")

	t.Run("non-self-attendee-detected", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{
			{Email: "alice@example.com"},
			{Email: "bob@vendor.io"},
		}}
		assert.True(t, e.HasExternalAttendees(me))
	})

	t.Run("all-attendees-are-me", func(t *testing.T) {
		multi := MakeIdentity([]string{"user@example.com", "alt@personal.io"}, "", "")
		e := CalendarEvent{Attendees: []Person{
			{Email: "user@example.com"},
			{Email: "alt@personal.io"},
		}}
		assert.False(t, e.HasExternalAttendees(multi))
	})

	t.Run("alt-email-treated-as-internal", func(t *testing.T) {
		multi := MakeIdentity([]string{"user@example.com", "alt@personal.io"}, "", "")
		e := CalendarEvent{Attendees: []Person{{Email: "alt@personal.io"}}}
		assert.False(t, e.HasExternalAttendees(multi))
	})

	t.Run("case-insensitive-match", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{{Email: "USER@example.com"}}}
		assert.False(t, e.HasExternalAttendees(me))
	})

	t.Run("empty-identity-emails-returns-false", func(t *testing.T) {
		empty := MakeIdentity(nil, "", "")
		e := CalendarEvent{Attendees: []Person{{Email: "anyone@vendor.io"}}}
		assert.False(t, e.HasExternalAttendees(empty))
	})

	t.Run("skips-empty-attendee-emails", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{
			{Name: "No Email"},
			{Email: "user@example.com"},
		}}
		// Only attendees with emails count, and the only one (user) is "me".
		assert.False(t, e.HasExternalAttendees(me))
	})

	t.Run("no-attendees", func(t *testing.T) {
		e := CalendarEvent{}
		assert.False(t, e.HasExternalAttendees(me))
	})
}
