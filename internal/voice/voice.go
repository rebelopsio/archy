package voice

// Voice controls personification across user-facing output. Set
// Enabled = false in --quiet mode and Signature = false when the
// caller doesn't want the trailing "— archy" line on generated
// marker blocks.
type Voice struct {
	// Enabled toggles first-person progress output. Disable for
	// --quiet mode and any machine-readable surface (--json, errors,
	// logs).
	Enabled bool
	// Signature toggles the "— archy" footer on marker-block content.
	// Independent of Enabled — a user can want the signature without
	// chatty progress, or vice versa.
	Signature bool
}

// Progress returns a first-person progress message with a trailing
// ellipsis when [Voice.Enabled] is true. Returns the empty string
// when disabled; callers should suppress output entirely (do not
// print empty strings — that produces blank lines).
func (v Voice) Progress(message string) string {
	if !v.Enabled {
		return ""
	}
	return message + "..."
}

// Sign returns the signature line ("\n\n— archy") when
// [Voice.Signature] is true, or the empty string otherwise. The
// renderer appends this to the block-composed body before the writer
// wraps it in marker comments.
func (v Voice) Sign() string {
	if !v.Signature {
		return ""
	}
	return "\n\n— archy"
}
