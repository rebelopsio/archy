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
	t.Run("external-domain-detected", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{
			{Email: "alice@example.com"},
			{Email: "bob@vendor.io"},
		}}
		assert.True(t, e.HasExternalAttendees("user@example.com"))
	})

	t.Run("all-internal", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{
			{Email: "alice@example.com"},
			{Email: "bob@example.com"},
		}}
		assert.False(t, e.HasExternalAttendees("user@example.com"))
	})

	t.Run("user-email-without-at", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{{Email: "alice@example.com"}}}
		assert.False(t, e.HasExternalAttendees("not-an-email"))
	})

	t.Run("case-insensitive-domain", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{{Email: "alice@EXAMPLE.com"}}}
		assert.False(t, e.HasExternalAttendees("user@example.com"))
	})

	t.Run("skips-empty-emails", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{
			{Name: "No Email"},
			{Email: "alice@example.com"},
		}}
		assert.False(t, e.HasExternalAttendees("user@example.com"))
	})

	t.Run("no-attendees", func(t *testing.T) {
		e := CalendarEvent{}
		assert.False(t, e.HasExternalAttendees("user@example.com"))
	})

	t.Run("user-email-trailing-at", func(t *testing.T) {
		e := CalendarEvent{Attendees: []Person{{Email: "alice@example.com"}}}
		assert.False(t, e.HasExternalAttendees("user@"))
	})
}
