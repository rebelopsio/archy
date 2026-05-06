package domain

// Person represents anyone referenced by an issue, PR, or calendar event.
// Kept minimal: enough to identify and display the person.
type Person struct {
	// Name is the display name. May be empty if the provider only gave
	// us an email or username.
	Name string

	// Email is the email address. May be empty.
	Email string

	// Username is the provider-specific handle (Linear username, GitHub
	// login, etc.). May be empty.
	Username string

	// IsBot is true if this person represents a bot account
	// (Renovate, Dependabot, GitHub Actions, etc.).
	IsBot bool
}

// String returns the best human-readable identifier available, in
// preference order: Name, Email, Username, "<unknown person>".
func (p Person) String() string {
	if p.Name != "" {
		return p.Name
	}
	if p.Email != "" {
		return p.Email
	}
	if p.Username != "" {
		return p.Username
	}
	return "<unknown person>"
}

// IsZero reports whether p has no identifying fields set. IsBot is not
// considered an identity field.
func (p Person) IsZero() bool {
	return p.Name == "" && p.Email == "" && p.Username == ""
}
