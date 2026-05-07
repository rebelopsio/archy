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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/version"
)

func TestVersionCmd_Registered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Use == "version" {
			return
		}
	}
	t.Fatal("version subcommand not registered on rootCmd")
}

func TestVersionCmd_PrintsVersionString(t *testing.T) {
	saveVersion, saveCommit, saveDate := version.Version, version.Commit, version.Date
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = saveVersion, saveCommit, saveDate
	})
	version.Version = "v9.9.9-test"
	version.Commit = "deadbeef"
	version.Date = "2026-05-07T00:00:00Z"

	buf := &bytes.Buffer{}
	versionCmd.SetOut(buf)
	versionCmd.Run(versionCmd, []string{})

	out := buf.String()
	assert.Contains(t, out, "v9.9.9-test")
	assert.Contains(t, out, "deadbeef")
	assert.Contains(t, out, "2026-05-07T00:00:00Z")
}

// TestVersionCmd_NoArgsValidator confirms the subcommand is structured
// to accept no positional args — there's no custom Args validator, and
// the Run function ignores its args slice. Bypasses cobra's dispatch
// entirely (Execute on a subcommand interacts with os.Args in ways
// that don't compose cleanly with the test binary's args).
func TestVersionCmd_NoArgsValidator(t *testing.T) {
	assert.Nil(t, versionCmd.Args, "version takes no required args; default validator is fine")

	buf := &bytes.Buffer{}
	versionCmd.SetOut(buf)
	versionCmd.Run(versionCmd, []string{})
	require.NotEmpty(t, buf.String(), "Run should emit version output")
}
