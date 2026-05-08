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
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rebelopsio/archy/internal/config"
	"github.com/rebelopsio/archy/internal/domain"
	"github.com/rebelopsio/archy/internal/mcpserver"
	"github.com/rebelopsio/archy/internal/scoring"
	"github.com/rebelopsio/archy/internal/write"
)

// mcpServerCmd is the hidden subcommand the agent runtime spawns as a
// child process. It speaks the MCP protocol over stdin/stdout. Per
// ADR-0003, the in-process MCP server pattern from ADR-0001 #4 is
// implemented as a subprocess of the same archy binary.
var mcpServerCmd = &cobra.Command{
	Use:    "mcp-server",
	Short:  "Run archy's MCP server (invoked by the agent runtime; not user-facing).",
	Hidden: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		cfg, err := config.LoadDefault()
		if err != nil {
			return err
		}

		writer, err := write.New(cfg.Vault.Path)
		if err != nil {
			return err
		}

		srv, err := mcpserver.New(mcpserver.Config{
			Writer:         writer,
			ScoringWeights: weightsFromConfig(cfg.Scoring),
			User:           domain.MakeIdentity(cfg.User.Emails, cfg.User.LinearHandle, cfg.User.GitHubHandle),
		})
		if err != nil {
			return err
		}

		return srv.Serve(ctx, mcpserver.NewStdioTransport())
	},
}

func init() {
	rootCmd.AddCommand(mcpServerCmd)
}

// weightsFromConfig maps the small config struct to the scoring struct.
// Currently covers only the three weight fields present in
// config.ScoringConfig; future additions extend this mapping.
func weightsFromConfig(s config.ScoringConfig) scoring.Weights {
	return scoring.Weights{
		MeetingSoon:     s.MeetingSoonWeight,
		UrgentIssue:     s.UrgentIssueWeight,
		ReviewRequested: s.ReviewRequestedWeight,
	}
}
