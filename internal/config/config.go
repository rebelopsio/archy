package config

import "time"

// Config is archy's runtime configuration, loaded from YAML and validated.
type Config struct {
	// Vault describes where archy writes notes and how its folders are named.
	Vault VaultConfig `mapstructure:"vault"`
	// MCPServers maps server name to its configuration.
	MCPServers map[string]MCPServerConfig `mapstructure:"mcp_servers"`
	// Skills configures skill discovery directories.
	Skills SkillsConfig `mapstructure:"skills"`
	// Agent configures the Claude Agent SDK runtime.
	Agent AgentConfig `mapstructure:"agent"`
	// Scoring holds tunable weights for the deterministic scoring engine.
	Scoring ScoringConfig `mapstructure:"scoring"`
	// State configures archy's local SQLite store.
	State StateConfig `mapstructure:"state"`
	// Output controls how archy renders and writes its output.
	Output OutputConfig `mapstructure:"output"`
}

// VaultConfig describes where archy writes notes and how its folders are
// named.
type VaultConfig struct {
	// Path is the absolute path to the user's vault root. Required. Must
	// be an existing directory at validation time.
	Path string `mapstructure:"path"`
	// Folders names the standard subdirectory layout inside the vault.
	Folders VaultFolders `mapstructure:"folders"`
}

// VaultFolders names the standard folder layout inside the vault.
// All values are relative to VaultConfig.Path and may not contain path
// separators or "..".
type VaultFolders struct {
	// Daily is the folder for daily-brief notes.
	Daily string `mapstructure:"daily"`
	// Meetings is the folder for meeting-prep notes.
	Meetings string `mapstructure:"meetings"`
	// Triage is the folder for triage notes.
	Triage string `mapstructure:"triage"`
	// Reviews is the folder for review-queue notes.
	Reviews string `mapstructure:"reviews"`
	// Inbox is the folder for ad-hoc capture.
	Inbox string `mapstructure:"inbox"`
}

// MCPServerConfig describes one external MCP server archy connects to.
type MCPServerConfig struct {
	// URL is the server's endpoint. Must use http or https when Enabled.
	URL string `mapstructure:"url"`
	// Enabled toggles the server. Disabled servers are not validated.
	Enabled bool `mapstructure:"enabled"`
}

// SkillsConfig points to the directories archy discovers skills from.
type SkillsConfig struct {
	// ProjectDir is the path to project-bundled skills. Defaults to
	// ".claude/skills" relative to the working directory.
	ProjectDir string `mapstructure:"project_dir"`
	// UserDir is the path to user-customized skills. Defaults to
	// "~/.claude/skills". Tilde is expanded at load time.
	UserDir string `mapstructure:"user_dir"`
}

// AgentConfig configures the Claude Agent SDK runtime.
type AgentConfig struct {
	// Model is the Claude model alias to use, e.g. "claude-sonnet-4-5".
	Model string `mapstructure:"model"`
	// MaxTurns caps the number of agent loop iterations per command.
	MaxTurns int `mapstructure:"max_turns"`
	// PermissionMode is one of "default", "acceptEdits", "bypassPermissions".
	PermissionMode string `mapstructure:"permission_mode"`
}

// ScoringConfig holds tunable weights for the deterministic scoring engine.
// Weights are non-negative integers; zero means the signal is disabled.
type ScoringConfig struct {
	// MeetingSoonWeight is added when a meeting is starting in the near future.
	MeetingSoonWeight int `mapstructure:"meeting_soon_weight"`
	// UrgentIssueWeight is added for issues with PriorityUrgent.
	UrgentIssueWeight int `mapstructure:"urgent_issue_weight"`
	// ReviewRequestedWeight is added when a PR review is requested from the user.
	ReviewRequestedWeight int `mapstructure:"review_requested_weight"`
}

// StateConfig configures archy's local SQLite store.
type StateConfig struct {
	// SQLitePath is the path to the state database. Tilde is expanded at
	// load time. Parent directory is created on first use, not at config
	// load time.
	SQLitePath string `mapstructure:"sqlite_path"`
	// CacheTTL is how long provider responses stay valid in the cache.
	// Zero means no caching.
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// OutputConfig controls how archy renders and writes its output.
type OutputConfig struct {
	// DefaultWriteMode is one of "marker-block", "overwrite", "append".
	DefaultWriteMode string `mapstructure:"default_write_mode"`
	// Timezone is an IANA timezone name, or "Local" for time.Local.
	Timezone string `mapstructure:"timezone"`
	// Signature controls whether generated marker blocks include a
	// "— archy" footer line.
	Signature bool `mapstructure:"signature"`
	// Voice controls whether progress messages use first-person voice.
	// Has no effect on JSON output or --quiet mode.
	Voice bool `mapstructure:"voice"`
}
