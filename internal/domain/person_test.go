package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPerson_String(t *testing.T) {
	cases := []struct {
		name   string
		person Person
		want   string
	}{
		{"name-set", Person{Name: "Ada Lovelace", Email: "ada@example.com", Username: "ada"}, "Ada Lovelace"},
		{"email-when-no-name", Person{Email: "ada@example.com", Username: "ada"}, "ada@example.com"},
		{"username-when-no-name-or-email", Person{Username: "ada"}, "ada"},
		{"unknown-when-zero", Person{}, "<unknown person>"},
		{"unknown-when-only-isbot", Person{IsBot: true}, "<unknown person>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.person.String())
		})
	}
}

func TestPerson_IsZero(t *testing.T) {
	cases := []struct {
		name   string
		person Person
		want   bool
	}{
		{"zero", Person{}, true},
		{"only-isbot", Person{IsBot: true}, true},
		{"only-name", Person{Name: "Ada"}, false},
		{"only-email", Person{Email: "a@b.c"}, false},
		{"only-username", Person{Username: "ada"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.person.IsZero())
		})
	}
}
