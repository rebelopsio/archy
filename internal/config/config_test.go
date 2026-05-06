package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearArchyEnv unsets every ARCHY_* env var the test harness might
// inherit from the developer's shell so per-test t.Setenv calls drive
// the entire env surface.
func clearArchyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ARCHY_VAULT_PATH",
		"ARCHY_VAULT_FOLDERS_DAILY",
		"ARCHY_AGENT_MODEL",
		"ARCHY_AGENT_MAX_TURNS",
		"ARCHY_AGENT_PERMISSION_MODE",
		"ARCHY_OUTPUT_VOICE",
		"ARCHY_OUTPUT_DEFAULT_WRITE_MODE",
		"ARCHY_OUTPUT_TIMEZONE",
		"ARCHY_STATE_CACHE_TTL",
		"ARCHY_STATE_SQLITE_PATH",
		"ARCHY_SKILLS_PROJECT_DIR",
		"ARCHY_SKILLS_USER_DIR",
		"ARCHY_SCORING_MEETING_SOON_WEIGHT",
	} {
		t.Setenv(k, "")
	}
}

// isolateConfigDir points os.UserConfigDir at a fresh tempdir for the
// test by setting HOME and XDG_CONFIG_HOME. Returns the resolved
// archy/config.yaml path.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	cfgDir, err := os.UserConfigDir()
	require.NoError(t, err)
	return filepath.Join(cfgDir, "archy", "config.yaml")
}

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// minimalConfig writes a config.yaml whose only set value is vault.path.
// The vault directory itself is created and returned.
func minimalConfig(t *testing.T) (yamlPath, vaultPath string) {
	t.Helper()
	dir := t.TempDir()
	vault := t.TempDir()
	yamlPath = filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, "vault:\n  path: "+vault+"\n")
	return yamlPath, vault
}

// === expandTilde ===

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde-slash-prefix", "~/foo", filepath.Join(home, "foo")},
		{"bare-tilde", "~", home},
		{"absolute-unchanged", "/abs/path", "/abs/path"},
		{"relative-unchanged", "relative/path", "relative/path"},
		{"deep-tilde-path", "~/foo/bar/baz", filepath.Join(home, "foo", "bar", "baz")},
		{"tilde-username-literal", "~user/foo", "~user/foo"},
		{"empty-string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandTilde(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// === Load ===

func TestLoad_FileNotFound(t *testing.T) {
	clearArchyEnv(t)
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConfigNotFound))
}

func TestLoad_MalformedYAML(t *testing.T) {
	clearArchyEnv(t)
	_, err := Load("testdata/malformed.yaml")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConfigParse), "expected ErrConfigParse, got %v", err)
}

func TestLoad_MinimalValid(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, vault := minimalConfig(t)

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, vault, cfg.Vault.Path)
	assert.Equal(t, "Daily", cfg.Vault.Folders.Daily)
	assert.Equal(t, "claude-sonnet-4-5", cfg.Agent.Model)
}

func TestLoad_PopulatesAllDefaults(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, vault := minimalConfig(t)

	cfg, err := Load(yamlPath)
	require.NoError(t, err)

	assert.Equal(t, vault, cfg.Vault.Path)
	assert.Equal(t, "Daily", cfg.Vault.Folders.Daily)
	assert.Equal(t, "Meetings", cfg.Vault.Folders.Meetings)
	assert.Equal(t, "Triage", cfg.Vault.Folders.Triage)
	assert.Equal(t, "Reviews", cfg.Vault.Folders.Reviews)
	assert.Equal(t, "Inbox", cfg.Vault.Folders.Inbox)
	assert.Empty(t, cfg.MCPServers)
	assert.Equal(t, ".claude/skills", cfg.Skills.ProjectDir)
	assert.Equal(t, 30, cfg.Agent.MaxTurns)
	assert.Equal(t, "acceptEdits", cfg.Agent.PermissionMode)
	assert.Equal(t, 5, cfg.Scoring.MeetingSoonWeight)
	assert.Equal(t, 8, cfg.Scoring.UrgentIssueWeight)
	assert.Equal(t, 7, cfg.Scoring.ReviewRequestedWeight)
	assert.Equal(t, 15*time.Minute, cfg.State.CacheTTL)
	assert.Equal(t, "marker-block", cfg.Output.DefaultWriteMode)
	assert.Equal(t, "Local", cfg.Output.Timezone)
	assert.True(t, cfg.Output.Signature)
	assert.True(t, cfg.Output.Voice)
}

func TestLoad_FileOverridesDefaults(t *testing.T) {
	clearArchyEnv(t)
	dir := t.TempDir()
	vault := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, `
vault:
  path: `+vault+`
  folders:
    daily: MyDaily
agent:
  model: claude-opus-4-7
  max_turns: 99
output:
  voice: false
  signature: false
state:
  cache_ttl: 1h
`)

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "MyDaily", cfg.Vault.Folders.Daily)
	assert.Equal(t, "Meetings", cfg.Vault.Folders.Meetings) // unchanged from default
	assert.Equal(t, "claude-opus-4-7", cfg.Agent.Model)
	assert.Equal(t, 99, cfg.Agent.MaxTurns)
	assert.False(t, cfg.Output.Voice)
	assert.False(t, cfg.Output.Signature)
	assert.Equal(t, time.Hour, cfg.State.CacheTTL)
}

func TestLoad_ExpandsTildes(t *testing.T) {
	clearArchyEnv(t)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, `
vault:
  path: ~/vault
skills:
  project_dir: ~/skills/project
  user_dir: ~/skills/user
state:
  sqlite_path: ~/db/state.db
`)
	// Pre-create the vault path so validation passes.
	require.NoError(t, os.MkdirAll(filepath.Join(home, "vault"), 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(home, "vault")) })

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "vault"), cfg.Vault.Path)
	assert.Equal(t, filepath.Join(home, "skills/project"), cfg.Skills.ProjectDir)
	assert.Equal(t, filepath.Join(home, "skills/user"), cfg.Skills.UserDir)
	assert.Equal(t, filepath.Join(home, "db/state.db"), cfg.State.SQLitePath)
}

// === Environment variable overrides ===

func TestEnv_ARCHY_VAULT_PATH_OverridesFile(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, _ := minimalConfig(t)

	// File specifies one vault; env points at another that must exist.
	envVault := t.TempDir()
	t.Setenv("ARCHY_VAULT_PATH", envVault)

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, envVault, cfg.Vault.Path)
}

func TestEnv_ARCHY_AGENT_MODEL_OverridesFile(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, _ := minimalConfig(t)

	t.Setenv("ARCHY_AGENT_MODEL", "claude-opus-4-7")
	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4-7", cfg.Agent.Model)
}

func TestEnv_ARCHY_OUTPUT_VOICE_OverridesFile(t *testing.T) {
	clearArchyEnv(t)
	dir := t.TempDir()
	vault := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, "vault:\n  path: "+vault+"\noutput:\n  voice: true\n")

	t.Setenv("ARCHY_OUTPUT_VOICE", "false")
	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.False(t, cfg.Output.Voice)
}

func TestEnv_ARCHY_STATE_CACHE_TTL_ParsesDuration(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, _ := minimalConfig(t)

	t.Setenv("ARCHY_STATE_CACHE_TTL", "30m")
	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, cfg.State.CacheTTL)
}

func TestEnv_AppliesWhenNoFileLoaded(t *testing.T) {
	clearArchyEnv(t)
	cfgFile := isolateConfigDir(t)
	require.NoFileExists(t, cfgFile)

	vault := t.TempDir()
	t.Setenv("ARCHY_VAULT_PATH", vault)

	cfg, err := LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, vault, cfg.Vault.Path)
}

// === LoadDefault ===

func TestLoadDefault_NoFile_ReturnsDefaults(t *testing.T) {
	clearArchyEnv(t)
	cfgFile := isolateConfigDir(t)
	require.NoFileExists(t, cfgFile)

	vault := t.TempDir()
	t.Setenv("ARCHY_VAULT_PATH", vault)

	cfg, err := LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, "Daily", cfg.Vault.Folders.Daily)
	assert.Equal(t, "claude-sonnet-4-5", cfg.Agent.Model)
}

func TestLoadDefault_LoadsFileAtDefaultLocation(t *testing.T) {
	clearArchyEnv(t)
	cfgFile := isolateConfigDir(t)
	vault := t.TempDir()
	writeYAML(t, cfgFile, "vault:\n  path: "+vault+"\nagent:\n  model: from-file\n")

	cfg, err := LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, vault, cfg.Vault.Path)
	assert.Equal(t, "from-file", cfg.Agent.Model)
}

func TestLoadDefault_RespectsXDGConfigHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME is honored only on Linux by os.UserConfigDir")
	}
	clearArchyEnv(t)

	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	vault := t.TempDir()
	cfgFile := filepath.Join(xdg, "archy", "config.yaml")
	writeYAML(t, cfgFile, "vault:\n  path: "+vault+"\nagent:\n  model: from-xdg\n")

	cfg, err := LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, "from-xdg", cfg.Agent.Model)
}

func TestLoadDefault_FallsBackToHomeConfig_WhenXDGUnset(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("~/.config fallback is Linux-specific in os.UserConfigDir")
	}
	clearArchyEnv(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	vault := t.TempDir()
	cfgFile := filepath.Join(home, ".config", "archy", "config.yaml")
	writeYAML(t, cfgFile, "vault:\n  path: "+vault+"\nagent:\n  model: from-home\n")

	cfg, err := LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, "from-home", cfg.Agent.Model)
}

// === Validate — passing ===

func TestValidate_FullyPopulatedValid(t *testing.T) {
	vault := t.TempDir()
	cfg := &Config{
		Vault: VaultConfig{Path: vault, Folders: VaultFolders{Daily: "Daily", Meetings: "Meetings", Triage: "Triage", Reviews: "Reviews", Inbox: "Inbox"}},
		MCPServers: map[string]MCPServerConfig{
			"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true},
		},
		Skills:  SkillsConfig{ProjectDir: ".claude/skills", UserDir: "/home/user/.claude/skills"},
		Agent:   AgentConfig{Model: "claude-sonnet-4-5", MaxTurns: 30, PermissionMode: "acceptEdits"},
		Scoring: ScoringConfig{MeetingSoonWeight: 5, UrgentIssueWeight: 8, ReviewRequestedWeight: 7},
		State:   StateConfig{SQLitePath: "/var/lib/archy/state.db", CacheTTL: 15 * time.Minute},
		Output:  OutputConfig{DefaultWriteMode: "marker-block", Timezone: "America/New_York", Signature: true, Voice: true},
	}
	require.NoError(t, cfg.Validate())
}

func TestValidate_MinimalDefaultsValid(t *testing.T) {
	clearArchyEnv(t)
	yamlPath, _ := minimalConfig(t)
	_, err := Load(yamlPath)
	require.NoError(t, err)
}

// === Validate — failing ===

// validBaseline returns a minimally valid Config we can mutate per test.
func validBaseline(t *testing.T) *Config {
	t.Helper()
	vault := t.TempDir()
	return &Config{
		Vault:   VaultConfig{Path: vault, Folders: VaultFolders{Daily: "Daily", Meetings: "Meetings", Triage: "Triage", Reviews: "Reviews", Inbox: "Inbox"}},
		Skills:  SkillsConfig{ProjectDir: ".claude/skills", UserDir: "/home/user/.claude/skills"},
		Agent:   AgentConfig{Model: "claude-sonnet-4-5", MaxTurns: 30, PermissionMode: "acceptEdits"},
		Scoring: ScoringConfig{MeetingSoonWeight: 5, UrgentIssueWeight: 8, ReviewRequestedWeight: 7},
		State:   StateConfig{SQLitePath: "/var/lib/archy/state.db", CacheTTL: 15 * time.Minute},
		Output:  OutputConfig{DefaultWriteMode: "marker-block", Timezone: "Local", Signature: true, Voice: true},
	}
}

func TestValidate_EmptyVaultPath(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Vault.Path = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "vault.path is required")
}

func TestValidate_NonexistentVaultPath(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Vault.Path = filepath.Join(t.TempDir(), "missing")
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "does not exist")
}

func TestValidate_VaultPathIsFile(t *testing.T) {
	cfg := validBaseline(t)
	target := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o644))
	cfg.Vault.Path = target

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "is not a directory")
}

func TestValidate_EmptyFolderName(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Vault.Folders.Daily = ""

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "vault.folders.daily is empty")
}

func TestValidate_FolderContainsDotDot(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Vault.Folders.Daily = "../escape"

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "vault.folders.daily")
}

func TestValidate_FolderContainsSlash(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Vault.Folders.Daily = "sub/folder"

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "vault.folders.daily")
}

func TestValidate_EnabledMCPServerNoURL(t *testing.T) {
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{"linear": {Enabled: true}}

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), `mcp_servers["linear"].url is required`)
}

func TestValidate_EnabledMCPServerBadScheme(t *testing.T) {
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{"linear": {URL: "ftp://bad", Enabled: true}}

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "must use http or https")
}

func TestValidate_DisabledMCPServerNotChecked(t *testing.T) {
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{"linear": {URL: "::malformed::", Enabled: false}}

	require.NoError(t, cfg.Validate())
}

func TestValidate_MaxTurnsZero(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Agent.MaxTurns = 0

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "agent.max_turns")
}

func TestValidate_PermissionModeUnknown(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Agent.PermissionMode = "bogus"

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "agent.permission_mode")
}

func TestValidate_NegativeScoringWeight(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Scoring.MeetingSoonWeight = -1

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "scoring.meeting_soon_weight")
}

func TestValidate_NegativeCacheTTL(t *testing.T) {
	cfg := validBaseline(t)
	cfg.State.CacheTTL = -1 * time.Second

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "state.cache_ttl")
}

func TestValidate_BadWriteMode(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Output.DefaultWriteMode = "bogus"

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "output.default_write_mode")
}

func TestValidate_BadTimezone(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Output.Timezone = "Mars/Olympus"

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "output.timezone")
}

func TestValidate_LocalTimezoneAccepted(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Output.Timezone = "Local"
	require.NoError(t, cfg.Validate())
}

func TestValidate_MultipleErrorsAllReported(t *testing.T) {
	cfg := validBaseline(t)
	cfg.Agent.MaxTurns = 0
	cfg.Output.DefaultWriteMode = "bogus"
	cfg.Scoring.UrgentIssueWeight = -10

	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	msg := err.Error()
	assert.Contains(t, msg, "agent.max_turns")
	assert.Contains(t, msg, "output.default_write_mode")
	assert.Contains(t, msg, "scoring.urgent_issue_weight")
}

// === BearerTokenEnv (per ADR-0006 / config-bearer-token PRD) ===

func TestLoad_BearerTokenEnvParses(t *testing.T) {
	clearArchyEnv(t)
	t.Setenv("ARCHY_LINEAR_TOKEN", "fake-token-value")

	dir := t.TempDir()
	vault := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, `
vault:
  path: `+vault+`
mcp_servers:
  linear:
    url: https://mcp.linear.app/mcp
    enabled: true
    bearer_token_env: ARCHY_LINEAR_TOKEN
`)

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	require.Contains(t, cfg.MCPServers, "linear")
	assert.Equal(t, "ARCHY_LINEAR_TOKEN", cfg.MCPServers["linear"].BearerTokenEnv)
}

func TestLoad_BearerTokenEnvAbsentIsEmpty(t *testing.T) {
	clearArchyEnv(t)
	dir := t.TempDir()
	vault := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	writeYAML(t, yamlPath, `
vault:
  path: `+vault+`
mcp_servers:
  linear:
    url: https://mcp.linear.app/mcp
    enabled: false
`)
	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	assert.Empty(t, cfg.MCPServers["linear"].BearerTokenEnv)
}

func TestValidate_BearerTokenEnv_AcceptsWhenEnvSet(t *testing.T) {
	t.Setenv("ARCHY_LINEAR_TOKEN", "fake-token-value")
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true, BearerTokenEnv: "ARCHY_LINEAR_TOKEN"},
	}
	require.NoError(t, cfg.Validate())
}

func TestValidate_BearerTokenEnv_RejectsWhenEnvEmpty(t *testing.T) {
	t.Setenv("ARCHY_LINEAR_TOKEN", "")
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true, BearerTokenEnv: "ARCHY_LINEAR_TOKEN"},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConfig))
	assert.Contains(t, err.Error(), "ARCHY_LINEAR_TOKEN")
	assert.Contains(t, err.Error(), "linear")
}

func TestValidate_BearerTokenEnv_DisabledServerSkipped(t *testing.T) {
	t.Setenv("ARCHY_LINEAR_TOKEN", "")
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: false, BearerTokenEnv: "ARCHY_LINEAR_TOKEN"},
	}
	require.NoError(t, cfg.Validate())
}

func TestValidate_BearerTokenEnv_EmptyOnEnabledServerOK(t *testing.T) {
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{
		// Enabled server, no bearer-token-env set — providers that don't
		// require auth (or are configured elsewhere) are valid.
		"some-public-mcp": {URL: "https://example.com/mcp", Enabled: true},
	}
	require.NoError(t, cfg.Validate())
}

func TestValidate_BearerTokenEnv_MultipleProvidersMixed(t *testing.T) {
	t.Setenv("ARCHY_LINEAR_TOKEN", "linear-token")
	// GitHub deliberately has no BearerTokenEnv — exercises the
	// "enabled but no auth field" branch alongside the "enabled with
	// auth" and "disabled with auth" branches.
	cfg := validBaseline(t)
	cfg.MCPServers = map[string]MCPServerConfig{
		"linear": {URL: "https://mcp.linear.app/mcp", Enabled: true, BearerTokenEnv: "ARCHY_LINEAR_TOKEN"},
		"github": {URL: "https://api.githubcopilot.com/mcp/", Enabled: true},
		"slack":  {URL: "https://example.com/slack", Enabled: false, BearerTokenEnv: "UNSET_BUT_DISABLED"},
	}
	require.NoError(t, cfg.Validate())
}
