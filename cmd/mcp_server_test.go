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
	"github.com/stretchr/testify/require"
)

func TestMCPServerCmd_Registered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Use == "mcp-server" {
			return
		}
	}
	t.Fatal("mcp-server subcommand not registered on rootCmd")
}

func TestMCPServerCmd_Hidden(t *testing.T) {
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Use == "mcp-server" {
			found = true
			assert.True(t, c.Hidden, "mcp-server should be hidden so it does not appear in --help")
			break
		}
	}
	require.True(t, found, "mcp-server subcommand not registered")
}

func TestWeightsFromConfig(t *testing.T) {
	// Sanity-check the small mapping helper. Not load-bearing on its
	// own, but the mapping is part of the agent-host wiring.
	got := weightsFromConfig(weightsTestInput())
	assert.Equal(t, 5, got.MeetingSoon)
	assert.Equal(t, 8, got.UrgentIssue)
	assert.Equal(t, 7, got.ReviewRequested)
}
