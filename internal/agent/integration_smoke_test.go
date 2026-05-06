//go:build integration

// Package agent's Tier 2 smoke test exercises the real subprocess
// pipeline end-to-end:
//
//	archy host (this test) → claude SDK → claude CLI → archy mcp-server → vault
//
// Run it manually with:
//
//	ARCHY_INTEGRATION_TEST=1 ARCHY_BINARY_PATH=$(which archy) \
//	    go test -tags=integration ./internal/agent/...
//
// CI does not run this test. Default `go test` does not run this test.
// It is the only place the full subprocess wiring is verified; if it
// breaks, every other package's tests still pass even though nothing
// works end-to-end.
//
// Skip conditions (in order):
//   - ARCHY_INTEGRATION_TEST != "1" — explicit opt-in.
//   - claude binary not in $PATH.
//   - ARCHY_BINARY_PATH not set, and "archy" not in $PATH.
package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/config"
)

// skillBody is a tiny SKILL.md body the smoke test installs in a
// temporary .claude/skills/integration-smoke/ directory.
const skillBody = `---
name: integration-smoke
description: Integration smoke test skill — writes one note and exits.
allowed-tools:
  - mcp__archy__archy_write_vault_note
---

You are running the archy integration smoke test.

Use ` + "`mcp__archy__archy_write_vault_note`" + ` to write a single note:

- path: ` + "`smoke.md`" + `
- mode: ` + "`marker-block`" + `
- marker_id: ` + "`integration-smoke`" + `
- content: ` + "`smoke ok`" + `

Then reply with one line confirming what you wrote.
`

func TestRun_IntegrationSmoke(t *testing.T) {
	if os.Getenv("ARCHY_INTEGRATION_TEST") != "1" {
		t.Skip("set ARCHY_INTEGRATION_TEST=1 to run integration smoke")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not in $PATH")
	}
	archyBin := os.Getenv("ARCHY_BINARY_PATH")
	if archyBin == "" {
		var err error
		archyBin, err = exec.LookPath("archy")
		if err != nil {
			t.Skip("set ARCHY_BINARY_PATH or put archy in $PATH to run integration smoke")
		}
	}

	vault := t.TempDir()
	skillsDir := filepath.Join(t.TempDir(), ".claude", "skills", "integration-smoke")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillBody), 0o644))

	cfg := &config.Config{
		Vault:  config.VaultConfig{Path: vault, Folders: config.VaultFolders{Daily: "Daily", Meetings: "Meetings", Triage: "Triage", Reviews: "Reviews", Inbox: "Inbox"}},
		Skills: config.SkillsConfig{ProjectDir: filepath.Dir(filepath.Dir(skillsDir)), UserDir: filepath.Join(t.TempDir(), ".claude", "skills")},
		Agent:  config.AgentConfig{Model: "claude-sonnet-4-5", MaxTurns: 5, PermissionMode: "acceptEdits"},
		State:  config.StateConfig{SQLitePath: filepath.Join(t.TempDir(), "state.db"), CacheTTL: time.Minute},
		Output: config.OutputConfig{DefaultWriteMode: "marker-block", Timezone: "Local"},
	}

	rt, err := New(Options{
		Config:          cfg,
		ArchyBinaryPath: archyBin,
		Cwd:             filepath.Dir(filepath.Dir(skillsDir)),
		UserEmail:       "smoke@example.com",
		UserUsername:    "smoke",
	})
	require.NoError(t, err)
	defer func() { _ = rt.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := rt.Run(ctx, RunRequest{
		SkillName: "integration-smoke",
		Prompt:    "Run the integration smoke skill.",
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	target := filepath.Join(vault, "smoke.md")
	body, err := os.ReadFile(target)
	require.NoError(t, err, "expected smoke.md to exist after the agent ran")
	assert.Contains(t, string(body), "smoke ok", "marker block content should land on disk")
	assert.Contains(t, string(body), "<!-- archy:start id=integration-smoke -->", "marker comment should be present")
}
