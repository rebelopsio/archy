/*
Copyright © 2026 Stephen Morgan <steve@rebelops.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/rebelopsio/archy/internal/agent"
	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/linear"
	"github.com/rebelopsio/archy/internal/render"
	"github.com/rebelopsio/archy/internal/scoring"
	"github.com/rebelopsio/archy/internal/state"
	"github.com/rebelopsio/archy/internal/voice"
)

// dailyTemplateName is the YAML file consulted for the daily-brief
// workflow. Resolved relative to the binary's working directory or
// ARCHY_TEMPLATE_DIR.
const dailyTemplateName = "daily.yaml"

var (
	dailyDryRun     bool
	dailyConfigPath string
)

// dailyCmd is the user-facing subcommand. archy daily gathers Linear
// issues, scores them, renders a markdown brief, and (unless --dry-run)
// hands the rendered body to the daily-brief skill which writes it to
// the vault via archy_write_vault_note.
var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Generate today's daily brief and write it to the vault.",
	Long: `daily gathers your open Linear issues, ranks them, and writes a
markdown brief to your vault under Daily/<today>.md.

Use --dry-run to preview the rendered body on stdout without invoking
the agent or writing to the vault.`,
	RunE: runDailyCommand,
}

func init() {
	dailyCmd.Flags().BoolVar(&dailyDryRun, "dry-run", false, "render the brief to stdout without writing or invoking the agent")
	dailyCmd.Flags().StringVar(&dailyConfigPath, "config", "", "config file path (default: $XDG_CONFIG_HOME/archy/config.yaml)")
	rootCmd.AddCommand(dailyCmd)
}

// runDailyCommand wires production dependencies and calls runDaily.
// Cobra-specific concerns (flag parsing, exit codes) live here; the
// orchestration logic is in daily_run.go.
func runDailyCommand(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := loadDailyConfig()
	if err != nil {
		return err
	}

	tpl, err := loadDailyTemplate()
	if err != nil {
		return err
	}

	registry, err := buildBlocksRegistry(tpl)
	if err != nil {
		return err
	}

	store, err := state.Open(ctx, cfg.State.SQLitePath)
	if err != nil {
		return fmt.Errorf("open state: %w", err)
	}
	defer func() { _ = store.Close() }()

	gatherer, err := newLinearGatherer(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = gatherer.Close() }()

	identity := domain.MakeIdentity(cfg.User.Emails, cfg.User.LinearHandle, cfg.User.GitHubHandle)

	scorer := runScorer{
		ctx: scoring.Context{
			Now:     time.Now(),
			User:    identity,
			Weights: weightsFromConfig(cfg.Scoring),
		},
	}

	var rt dailyRuntime
	if !dailyDryRun {
		rt, err = agent.New(agent.Options{
			Config: cfg,
			User:   identity,
		})
		if err != nil {
			return fmt.Errorf("init agent runtime: %w", err)
		}
		defer func() { _ = rt.Close() }()
	}

	deps := dailyDeps{
		cfg:           cfg,
		template:      tpl,
		registry:      registry,
		scorer:        scorer,
		issueGatherer: gatherer,
		runtime:       rt,
		store:         store,
		voice:         voice.Voice{Enabled: cfg.Output.Voice, Signature: cfg.Output.Signature},
		now:           time.Now,
		stdout:        cmd.OutOrStdout(),
	}

	res, err := runDaily(ctx, deps, dailyOptions{DryRun: dailyDryRun})
	if err != nil {
		return err
	}

	switch {
	case res.Skipped:
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), res.SkipReason)
	case dailyDryRun:
		// Body already printed by runDaily.
	case res.AgentResult != nil:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", res.TargetPath)
	}
	return nil
}

// loadDailyConfig honors the --config flag when set, falling back to
// the default location.
func loadDailyConfig() (*config.Config, error) {
	if dailyConfigPath != "" {
		cfg, err := config.Load(dailyConfigPath)
		if err != nil {
			return nil, fmt.Errorf("load config %s: %w", dailyConfigPath, err)
		}
		return cfg, nil
	}
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load default config: %w", err)
	}
	return cfg, nil
}

// loadDailyTemplate finds templates/daily.yaml relative to the
// binary's working directory, with ARCHY_TEMPLATE_DIR as an override.
// Returns a clear error when not found.
func loadDailyTemplate() (*render.Template, error) {
	dir := os.Getenv("ARCHY_TEMPLATE_DIR")
	if dir == "" {
		dir = "templates"
	}
	path := filepath.Join(dir, dailyTemplateName)
	tpl, err := render.LoadTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("load template (set ARCHY_TEMPLATE_DIR if not in templates/): %w", err)
	}
	return tpl, nil
}

// newLinearGatherer constructs a *linear.Client from the Linear entry
// in cfg.MCPServers, reading the bearer token from the configured env
// var. Returns an error when Linear is not enabled, missing, or
// missing its bearer-token-env value.
func newLinearGatherer(cfg *config.Config) (*linear.Client, error) {
	srv, ok := cfg.MCPServers["linear"]
	if !ok || !srv.Enabled {
		return nil, fmt.Errorf("daily: linear is not enabled in mcp_servers; configure mcp_servers.linear in config")
	}
	if srv.BearerTokenEnv == "" {
		return nil, fmt.Errorf("daily: linear has no bearer_token_env set; configure it in config")
	}
	token := os.Getenv(srv.BearerTokenEnv)
	if token == "" {
		// The validator should have caught this already, but defend in depth.
		return nil, fmt.Errorf("daily: %s is empty", srv.BearerTokenEnv)
	}
	return linear.New(linear.Config{URL: srv.URL, BearerToken: token})
}
