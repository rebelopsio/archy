package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeIdentity_LowercasesAllFields(t *testing.T) {
	id := MakeIdentity(
		[]string{"Steve@Rebelops.IO", "alt@PERSONAL.example"},
		"Steve",
		"Rebelopsio",
	)
	assert.Equal(t, []string{"steve@rebelops.io", "alt@personal.example"}, id.Emails)
	assert.Equal(t, "steve", id.LinearHandle)
	assert.Equal(t, "rebelopsio", id.GitHubHandle)
}

func TestMakeIdentity_PreservesEmailOrder(t *testing.T) {
	id := MakeIdentity([]string{"second@x.com", "first@y.com", "third@z.com"}, "", "")
	assert.Equal(t, "second@x.com", id.Emails[0], "first entry of input remains primary")
	assert.Equal(t, []string{"second@x.com", "first@y.com", "third@z.com"}, id.Emails)
}

func TestMakeIdentity_EmptyEmailsYieldsNilSlice(t *testing.T) {
	id := MakeIdentity(nil, "steve", "")
	assert.Nil(t, id.Emails)
	assert.Equal(t, "steve", id.LinearHandle)
}

func TestIdentity_MatchesEmail(t *testing.T) {
	id := MakeIdentity([]string{"primary@example.com", "alt@personal.io"}, "", "")

	t.Run("primary-matches", func(t *testing.T) {
		assert.True(t, id.MatchesEmail("primary@example.com"))
	})
	t.Run("alt-matches", func(t *testing.T) {
		assert.True(t, id.MatchesEmail("alt@personal.io"))
	})
	t.Run("case-insensitive", func(t *testing.T) {
		assert.True(t, id.MatchesEmail("PRIMARY@example.com"))
		assert.True(t, id.MatchesEmail("Alt@Personal.IO"))
	})
	t.Run("non-member", func(t *testing.T) {
		assert.False(t, id.MatchesEmail("stranger@vendor.com"))
	})
	t.Run("empty-input", func(t *testing.T) {
		assert.False(t, id.MatchesEmail(""))
	})
}

func TestIdentity_MatchesLinearHandle(t *testing.T) {
	id := MakeIdentity([]string{"u@e.com"}, "Steve", "")

	t.Run("exact-match", func(t *testing.T) {
		assert.True(t, id.MatchesLinearHandle("steve"))
	})
	t.Run("case-insensitive", func(t *testing.T) {
		assert.True(t, id.MatchesLinearHandle("STEVE"))
		assert.True(t, id.MatchesLinearHandle("Steve"))
	})
	t.Run("different-handle", func(t *testing.T) {
		assert.False(t, id.MatchesLinearHandle("alice"))
	})
	t.Run("empty-candidate", func(t *testing.T) {
		assert.False(t, id.MatchesLinearHandle(""))
	})

	t.Run("empty-identity-handle-never-matches", func(t *testing.T) {
		idEmpty := MakeIdentity([]string{"u@e.com"}, "", "")
		assert.False(t, idEmpty.MatchesLinearHandle("steve"))
		assert.False(t, idEmpty.MatchesLinearHandle(""))
	})
}

func TestIdentity_MatchesGitHubHandle(t *testing.T) {
	id := MakeIdentity([]string{"u@e.com"}, "", "rebelopsio")

	assert.True(t, id.MatchesGitHubHandle("rebelopsio"))
	assert.True(t, id.MatchesGitHubHandle("REBELOPSIO"))
	assert.False(t, id.MatchesGitHubHandle("octocat"))
	assert.False(t, id.MatchesGitHubHandle(""))

	idEmpty := MakeIdentity([]string{"u@e.com"}, "", "")
	assert.False(t, idEmpty.MatchesGitHubHandle("rebelopsio"))
}
