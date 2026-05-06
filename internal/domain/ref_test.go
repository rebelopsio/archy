package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExternalRef_String_Populated(t *testing.T) {
	r := ExternalRef{Provider: "linear", ID: "LIN-123"}
	assert.Equal(t, "linear:LIN-123", r.String())
}

func TestExternalRef_String_Zero(t *testing.T) {
	assert.Equal(t, "<unknown>", ExternalRef{}.String())
}

func TestExternalRef_String_OmitsURL(t *testing.T) {
	r := ExternalRef{Provider: "github", ID: "owner/repo#456", URL: "https://github.com/owner/repo/pull/456"}
	assert.Equal(t, "github:owner/repo#456", r.String())
}

func TestExternalRef_IsZero(t *testing.T) {
	cases := []struct {
		name string
		ref  ExternalRef
		want bool
	}{
		{"zero", ExternalRef{}, true},
		{"only-provider", ExternalRef{Provider: "linear"}, false},
		{"only-id", ExternalRef{ID: "LIN-1"}, false},
		{"only-url-treated-as-zero", ExternalRef{URL: "https://example.com"}, true},
		{"populated", ExternalRef{Provider: "linear", ID: "LIN-1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.ref.IsZero())
		})
	}
}
