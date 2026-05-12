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

// Package cmd implements the archy CLI.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/rebelopsio/archy/internal/version"
)

// rootCmd is the entry point for the archy binary. Subcommands register
// themselves on it from sibling files (daily.go, mcp_server.go, version.go).
var rootCmd = &cobra.Command{
	Use:   "archy",
	Short: "Personal AI assistant for a markdown vault",
	Long: `archy is a personal AI assistant for managing a markdown vault from
Neovim and the CLI. Built on the Claude Agent SDK, it orchestrates Linear,
GitHub, Google Calendar, and other systems into structured Obsidian-vault
notes — daily briefs, meeting prep, review queues, triage, capture — via
composable Skills, MCP servers, and typed blocks.

Configuration lives at $XDG_CONFIG_HOME/archy/config.yaml (or
~/.config/archy/config.yaml when XDG_CONFIG_HOME is unset). Local state
is kept in a SQLite database under $XDG_DATA_HOME/archy/state.db.

See https://github.com/rebelopsio/archy for documentation.`,
	// Version enables cobra's built-in --version flag. The custom
	// template strips cobra's default "archy version " prefix so
	// `archy --version` produces the same output as `archy version`.
	Version: version.String(),
}

// Execute runs the archy CLI. It is called by main.main() and exits the
// process with a non-zero status if a subcommand returns an error.
// Cobra prints the error itself before returning.
func Execute() {
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
