package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

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
	// BearerTokenEnv names the environment variable holding the
	// provider's bearer token (a personal access token or OAuth access
	// token). archy reads the env var at startup and uses the value as
	// "Authorization: Bearer <value>" for both archy's direct Go MCP
	// client (orchestrator path, per ADR-0006) and the agent SDK's MCP
	// server config (skill path).
	//
	// Optional. A provider that does not require authentication can
	// leave this empty. If set on an enabled server, the named env
	// variable must be non-empty at validation time; the validator
	// rejects "set but empty" misconfigurations at startup.
	BearerTokenEnv string `mapstructure:"bearer_token_env,omitempty"`
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

// Load reads the config from path, applies defaults, applies environment
// variable overrides, validates, and returns a Config. Returns
// ErrConfigNotFound (wrapped) when the file does not exist,
// ErrConfigParse (wrapped) when the YAML is malformed, and
// ErrInvalidConfig (wrapped) when validation fails.
func Load(path string) (*Config, error) {
	v := newViper()

	f, err := os.Open(path) //nolint:gosec // path is operator-supplied; reading config files is the package's job
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load %s: %w", path, ErrConfigNotFound)
		}
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := v.ReadConfig(f); err != nil {
		return nil, fmt.Errorf("load %s: %w: %v", path, ErrConfigParse, err)
	}
	return finalize(v)
}

// LoadDefault loads the config from the default location:
// $XDG_CONFIG_HOME/archy/config.yaml on Linux (or ~/.config/archy/config.yaml
// when XDG_CONFIG_HOME is unset), ~/Library/Application Support/archy/config.yaml
// on macOS. If the file does not exist, returns a Config populated entirely
// from defaults (and any ARCHY_ env-var overrides) and validates it. Returns
// an error only on I/O, parse, or validation failure.
func LoadDefault() (*Config, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("load default: %w", err)
	}
	path := filepath.Join(cfgDir, "archy", "config.yaml")

	v := newViper()

	f, err := os.Open(path) //nolint:gosec // default config path computed from os.UserConfigDir
	switch {
	case err == nil:
		defer func() { _ = f.Close() }()
		if err := v.ReadConfig(f); err != nil {
			return nil, fmt.Errorf("load default %s: %w: %v", path, ErrConfigParse, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// fall through: defaults + env vars only
	default:
		return nil, fmt.Errorf("load default %s: %w", path, err)
	}
	return finalize(v)
}

// newViper returns a Viper instance pre-configured with archy's defaults
// and ARCHY_-prefixed env-var binding. Each call returns a fresh instance
// so concurrent loads do not interfere.
func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ARCHY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)
	return v
}

// finalize unmarshals v into a Config, expands tildes in path-bearing
// fields, and validates.
func finalize(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := expandConfigTildes(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// setDefaults registers archy's default values with v. The empty default
// for vault.path is deliberate: it has no functional default (the user
// must set one) but registering it makes ARCHY_VAULT_PATH apply during
// Unmarshal.
func setDefaults(v *viper.Viper) {
	v.SetDefault("vault.path", "")
	v.SetDefault("vault.folders.daily", "Daily")
	v.SetDefault("vault.folders.meetings", "Meetings")
	v.SetDefault("vault.folders.triage", "Triage")
	v.SetDefault("vault.folders.reviews", "Reviews")
	v.SetDefault("vault.folders.inbox", "Inbox")
	v.SetDefault("mcp_servers", map[string]any{})
	v.SetDefault("skills.project_dir", ".claude/skills")
	v.SetDefault("skills.user_dir", "~/.claude/skills")
	v.SetDefault("agent.model", "claude-sonnet-4-5")
	v.SetDefault("agent.max_turns", 30)
	v.SetDefault("agent.permission_mode", "acceptEdits")
	v.SetDefault("scoring.meeting_soon_weight", 5)
	v.SetDefault("scoring.urgent_issue_weight", 8)
	v.SetDefault("scoring.review_requested_weight", 7)
	v.SetDefault("state.sqlite_path", "~/.local/share/archy/state.db")
	v.SetDefault("state.cache_ttl", 15*time.Minute)
	v.SetDefault("output.default_write_mode", "marker-block")
	v.SetDefault("output.timezone", "Local")
	v.SetDefault("output.signature", true)
	v.SetDefault("output.voice", true)
}

// expandConfigTildes expands "~/" and bare "~" in the four path-bearing
// fields. Errors propagate the os.UserHomeDir failure, if any.
func expandConfigTildes(c *Config) error {
	var err error
	if c.Vault.Path, err = expandTilde(c.Vault.Path); err != nil {
		return err
	}
	if c.Skills.ProjectDir, err = expandTilde(c.Skills.ProjectDir); err != nil {
		return err
	}
	if c.Skills.UserDir, err = expandTilde(c.Skills.UserDir); err != nil {
		return err
	}
	if c.State.SQLitePath, err = expandTilde(c.State.SQLitePath); err != nil {
		return err
	}
	return nil
}

// expandTilde expands a leading "~/" or bare "~" in path to the user's
// home directory. Returns path unchanged otherwise.
//
// User-name expansion is intentionally not supported: "~user/foo" is
// treated as a literal path starting with "~user/", consistent with how
// many other Go tools handle this.
func expandTilde(path string) (string, error) {
	if path == "" {
		return path, nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// Validate returns an error if c is not a usable configuration. All
// validation failures are collected and joined via errors.Join so callers
// see every problem at once. Each underlying error wraps ErrInvalidConfig.
func (c *Config) Validate() error {
	var errs []error

	c.validateVault(&errs)
	c.validateMCPServers(&errs)
	c.validateSkills(&errs)
	c.validateAgent(&errs)
	c.validateScoring(&errs)
	c.validateState(&errs)
	c.validateOutput(&errs)

	return errors.Join(errs...)
}

func (c *Config) validateVault(errs *[]error) {
	if c.Vault.Path == "" {
		*errs = append(*errs, fmt.Errorf("vault.path is required: %w", ErrInvalidConfig))
	} else if info, err := os.Stat(c.Vault.Path); err != nil {
		*errs = append(*errs, fmt.Errorf("vault.path %q does not exist: %w", c.Vault.Path, ErrInvalidConfig))
	} else if !info.IsDir() {
		*errs = append(*errs, fmt.Errorf("vault.path %q is not a directory: %w", c.Vault.Path, ErrInvalidConfig))
	}

	folders := []struct {
		name, val string
	}{
		{"daily", c.Vault.Folders.Daily},
		{"meetings", c.Vault.Folders.Meetings},
		{"triage", c.Vault.Folders.Triage},
		{"reviews", c.Vault.Folders.Reviews},
		{"inbox", c.Vault.Folders.Inbox},
	}
	for _, f := range folders {
		if f.val == "" {
			*errs = append(*errs, fmt.Errorf("vault.folders.%s is empty: %w", f.name, ErrInvalidConfig))
			continue
		}
		if strings.Contains(f.val, "..") || strings.ContainsAny(f.val, `/\`) {
			*errs = append(*errs, fmt.Errorf("vault.folders.%s %q must not contain path separators or '..': %w", f.name, f.val, ErrInvalidConfig))
		}
	}
}

func (c *Config) validateMCPServers(errs *[]error) {
	for name, srv := range c.MCPServers {
		if !srv.Enabled {
			continue
		}
		if srv.URL == "" {
			*errs = append(*errs, fmt.Errorf("mcp_servers[%q].url is required when enabled: %w", name, ErrInvalidConfig))
			continue
		}
		u, err := url.Parse(srv.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			*errs = append(*errs, fmt.Errorf("mcp_servers[%q].url %q must use http or https scheme: %w", name, srv.URL, ErrInvalidConfig))
		}

		// Bearer-token env-var presence check. This is unusual for a
		// validator (which is otherwise a pure function of the config),
		// but catching this misconfiguration at startup avoids a
		// confusing mid-run failure when archy tries to authenticate.
		// Documented as deliberate so future readers do not "purify" it.
		if srv.BearerTokenEnv != "" && os.Getenv(srv.BearerTokenEnv) == "" {
			*errs = append(*errs, fmt.Errorf(
				"mcp_servers[%q]: bearer_token_env %q is set, but the named env var is empty: %w",
				name, srv.BearerTokenEnv, ErrInvalidConfig,
			))
		}
	}
}

func (c *Config) validateSkills(errs *[]error) {
	if c.Skills.ProjectDir == "" {
		*errs = append(*errs, fmt.Errorf("skills.project_dir is empty: %w", ErrInvalidConfig))
	}
	if c.Skills.UserDir == "" {
		*errs = append(*errs, fmt.Errorf("skills.user_dir is empty: %w", ErrInvalidConfig))
	}
}

func (c *Config) validateAgent(errs *[]error) {
	if c.Agent.Model == "" {
		*errs = append(*errs, fmt.Errorf("agent.model is empty: %w", ErrInvalidConfig))
	}
	if c.Agent.MaxTurns <= 0 {
		*errs = append(*errs, fmt.Errorf("agent.max_turns must be > 0, got %d: %w", c.Agent.MaxTurns, ErrInvalidConfig))
	}
	switch c.Agent.PermissionMode {
	case "default", "acceptEdits", "bypassPermissions":
	default:
		*errs = append(*errs, fmt.Errorf("agent.permission_mode %q must be one of default, acceptEdits, bypassPermissions: %w", c.Agent.PermissionMode, ErrInvalidConfig))
	}
}

func (c *Config) validateScoring(errs *[]error) {
	weights := []struct {
		name string
		val  int
	}{
		{"meeting_soon_weight", c.Scoring.MeetingSoonWeight},
		{"urgent_issue_weight", c.Scoring.UrgentIssueWeight},
		{"review_requested_weight", c.Scoring.ReviewRequestedWeight},
	}
	for _, w := range weights {
		if w.val < 0 {
			*errs = append(*errs, fmt.Errorf("scoring.%s must be >= 0, got %d: %w", w.name, w.val, ErrInvalidConfig))
		}
	}
}

func (c *Config) validateState(errs *[]error) {
	if c.State.SQLitePath == "" {
		*errs = append(*errs, fmt.Errorf("state.sqlite_path is empty: %w", ErrInvalidConfig))
	}
	if c.State.CacheTTL < 0 {
		*errs = append(*errs, fmt.Errorf("state.cache_ttl must be >= 0, got %s: %w", c.State.CacheTTL, ErrInvalidConfig))
	}
}

func (c *Config) validateOutput(errs *[]error) {
	switch c.Output.DefaultWriteMode {
	case "marker-block", "overwrite", "append":
	default:
		*errs = append(*errs, fmt.Errorf("output.default_write_mode %q must be one of marker-block, overwrite, append: %w", c.Output.DefaultWriteMode, ErrInvalidConfig))
	}
	if _, err := time.LoadLocation(c.Output.Timezone); err != nil {
		*errs = append(*errs, fmt.Errorf("output.timezone %q is not loadable via time.LoadLocation: %w", c.Output.Timezone, ErrInvalidConfig))
	}
}
