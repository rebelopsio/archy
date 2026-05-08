package domain

import "strings"

// Identity is the operator's cross-provider identity — the lowercased,
// normalized form of the user-identity config that scoring signals and
// the agent runtime use to recognize "me".
//
// The fields are lowercased once at construction time. Downstream code
// performs case-insensitive matching by lowercasing the candidate value
// and comparing against the already-lowercased identity fields, rather
// than re-lowercasing the identity on every comparison.
type Identity struct {
	// Emails holds every email address that should be treated as "me".
	// The first entry is the primary, used for any single-value
	// attribution. Always non-empty for a valid Identity.
	Emails []string

	// LinearHandle is the operator's Linear username, lowercased. May
	// be empty when Linear is not configured.
	LinearHandle string

	// GitHubHandle is the operator's GitHub username, lowercased. May
	// be empty when GitHub is not configured.
	GitHubHandle string
}

// MakeIdentity returns an Identity with all string fields lowercased.
// The inputs are assumed to have passed config validation; this
// constructor does not re-validate. Pass the values directly from a
// validated config.UserConfig.
func MakeIdentity(emails []string, linearHandle, githubHandle string) Identity {
	out := Identity{
		LinearHandle: strings.ToLower(linearHandle),
		GitHubHandle: strings.ToLower(githubHandle),
	}
	if len(emails) > 0 {
		out.Emails = make([]string, len(emails))
		for i, e := range emails {
			out.Emails[i] = strings.ToLower(e)
		}
	}
	return out
}

// MatchesEmail reports whether addr (case-insensitive) is one of the
// operator's registered emails.
func (i Identity) MatchesEmail(addr string) bool {
	if addr == "" {
		return false
	}
	target := strings.ToLower(addr)
	for _, e := range i.Emails {
		if e == target {
			return true
		}
	}
	return false
}

// MatchesLinearHandle reports whether handle (case-insensitive) matches
// the operator's Linear handle. Returns false if the operator has no
// Linear handle configured.
func (i Identity) MatchesLinearHandle(handle string) bool {
	if i.LinearHandle == "" || handle == "" {
		return false
	}
	return i.LinearHandle == strings.ToLower(handle)
}

// MatchesGitHubHandle reports whether handle (case-insensitive) matches
// the operator's GitHub handle. Returns false if the operator has no
// GitHub handle configured.
func (i Identity) MatchesGitHubHandle(handle string) bool {
	if i.GitHubHandle == "" || handle == "" {
		return false
	}
	return i.GitHubHandle == strings.ToLower(handle)
}
