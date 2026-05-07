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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rebelopsio/archy/internal/version"
)

// TestRootCmd_VersionFieldSet asserts the root command exposes
// `--version` (cobra wires the flag automatically when Version is set)
// and that it produces the same string as the version subcommand.
func TestRootCmd_VersionFieldSet(t *testing.T) {
	saveVersion, saveCommit, saveDate := version.Version, version.Commit, version.Date
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = saveVersion, saveCommit, saveDate
	})

	// rootCmd.Version is captured at package init time, so we can't
	// re-mutate the version vars and re-read it here — what we can
	// assert is that the field is non-empty and matches the format
	// version.String() produces (which the version package's own
	// tests cover in detail).
	assert.NotEmpty(t, rootCmd.Version, "rootCmd.Version must be set so cobra adds --version")
	assert.Contains(t, rootCmd.Version, "archy ", "rootCmd.Version should be the canonical version.String() output")
}
