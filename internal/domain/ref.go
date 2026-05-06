package domain

// ExternalRef identifies an entity in a specific external provider's system.
// Domain types embed an ExternalRef instead of carrying provider-specific
// ID fields; provider quirks never leak into the domain.
type ExternalRef struct {
	// Provider is a stable string identifying the source system, e.g.
	// "linear", "github", "google_calendar". Lowercase, snake_case if
	// multi-word.
	Provider string

	// ID is the provider-specific identifier as a string. Format and
	// meaning are opaque to the domain — they're whatever the provider
	// returns. Examples: "LIN-123", "owner/repo#456", a UUID, an integer.
	ID string

	// URL is the canonical web URL for this entity, if available.
	// Optional; empty string means none. Used for display and wikilink
	// generation, never for identity.
	URL string
}

// String returns "<provider>:<id>" for a populated ref, or "<unknown>"
// for the zero value. URL is not included in the output.
func (r ExternalRef) String() string {
	if r.IsZero() {
		return "<unknown>"
	}
	return r.Provider + ":" + r.ID
}

// IsZero reports whether r has no Provider and no ID. URL is not
// considered — a ref with only a URL is malformed and treated as zero.
func (r ExternalRef) IsZero() bool {
	return r.Provider == "" && r.ID == ""
}
