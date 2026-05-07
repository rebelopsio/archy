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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rebelopsio/archy/internal/version"
)

// versionCmd prints archy's build metadata. The values are populated
// via -ldflags at release-build time; development builds show
// "dev" / "unknown" defaults.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print archy's version, build commit, and build date",
	Long: `Print archy's version, build commit, and build date.

Useful for bug reports and for the future ` + "`archy doctor`" + ` health
check, which embeds this output. Development builds show "dev" and
"unknown" for the three values; release builds embed real values via
goreleaser's -ldflags.`,
	Run: func(cmd *cobra.Command, _ []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), version.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
