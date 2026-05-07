package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString_DefaultValues(t *testing.T) {
	// In a fresh test binary the package vars hold their defaults
	// because no -ldflags were applied. Capture the current values
	// before mutation in case earlier tests changed them.
	saveVersion, saveCommit, saveDate := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = saveVersion, saveCommit, saveDate })

	Version, Commit, Date = "dev", "unknown", "unknown"
	assert.Equal(t, "archy dev (unknown, built unknown)", String())
}

func TestString_ReflectsSetValues(t *testing.T) {
	saveVersion, saveCommit, saveDate := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = saveVersion, saveCommit, saveDate })

	Version = "v1.2.3"
	Commit = "abcd1234"
	Date = "2026-05-07T10:00:00Z"
	got := String()

	assert.Contains(t, got, "v1.2.3")
	assert.Contains(t, got, "abcd1234")
	assert.Contains(t, got, "2026-05-07T10:00:00Z")
	assert.True(t, strings.HasPrefix(got, "archy "))
}
