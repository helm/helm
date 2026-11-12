/*
Copyright The Helm Authors.

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
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayp/ophis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
)

type mcpTestTool struct {
	Name        string `json:"name"`
	Annotations struct {
		ReadOnlyHint    bool  `json:"readOnlyHint"`
		DestructiveHint *bool `json:"destructiveHint"`
	} `json:"annotations"`
	InputSchema struct {
		Properties map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"properties"`
	} `json:"inputSchema"`
}

func exportMCPTools(t *testing.T, allowWrite bool) []mcpTestTool {
	t.Helper()
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	args := []string{"mcp", "tools"}
	if allowWrite {
		args = append(args, "--allow-write")
	}
	root, err := newRootCmdWithConfig(action.NewConfiguration(), &out, args, SetupLogging)
	require.NoError(t, err)
	root.SetArgs(args)
	require.NoError(t, root.Execute())

	data, err := os.ReadFile("mcp-tools.json")
	require.NoError(t, err)
	var tools []mcpTestTool
	require.NoError(t, json.Unmarshal(data, &tools))
	return tools
}

func TestMCPTools(t *testing.T) {
	for _, tt := range []struct {
		name       string
		allowWrite bool
	}{
		{"read-only by default", false},
		{"write enabled", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tools := exportMCPTools(t, tt.allowWrite)
			expectedCount := len(mcpReadCommands)
			if tt.allowWrite {
				expectedCount += len(mcpWriteCommands)
			}
			require.Len(t, tools, expectedCount)

			var installFlags, upgradeFlags map[string]json.RawMessage
			seen := make(map[string]bool)
			for _, tool := range tools {
				path := strings.ReplaceAll(tool.Name, "_", " ")
				assert.False(t, seen[path], "duplicate MCP tool %q", path)
				seen[path] = true
				if slices.Contains(mcpWriteCommands, path) {
					assert.True(t, tt.allowWrite, "write tool available by default: %s", path)
					assert.False(t, tool.Annotations.ReadOnlyHint, tool.Name)
					if assert.NotNil(t, tool.Annotations.DestructiveHint, tool.Name) {
						assert.True(t, *tool.Annotations.DestructiveHint, tool.Name)
					}
				} else {
					assert.Contains(t, mcpReadCommands, path)
					assert.True(t, tool.Annotations.ReadOnlyHint, tool.Name)
				}
				flags := tool.InputSchema.Properties["flags"].Properties
				assert.NotContains(t, flags, "kube-token", tool.Name)
				assert.NotContains(t, flags, "kube-as-user", tool.Name)
				assert.NotContains(t, flags, "debug", tool.Name)
				assert.NotContains(t, flags, "post-renderer", tool.Name)
				assert.NotContains(t, flags, "post-renderer-args", tool.Name)
				assert.Contains(t, flags, "namespace", tool.Name)
				switch tool.Name {
				case "helm_install":
					installFlags = flags
				case "helm_upgrade":
					upgradeFlags = flags
				}
			}
			for _, path := range mcpReadCommands {
				assert.True(t, seen[path], "missing MCP tool %q", path)
			}
			for _, path := range mcpWriteCommands {
				assert.Equal(t, tt.allowWrite, seen[path], "MCP tool %q", path)
			}
			if tt.allowWrite {
				require.NotNil(t, installFlags)
				assert.Contains(t, installFlags, "dry-run")
				require.NotNil(t, upgradeFlags)
				assert.Contains(t, upgradeFlags, "install")
				assert.Contains(t, upgradeFlags, "dry-run")
			}

			var showFlags map[string]json.RawMessage
			for _, tool := range tools {
				if tool.Name == "helm_show_chart" {
					showFlags = tool.InputSchema.Properties["flags"].Properties
				}
			}
			require.NotNil(t, showFlags)
			assert.NotContains(t, showFlags, "password")
			assert.NotContains(t, showFlags, "pass-credentials")
			assert.NotContains(t, showFlags, "insecure-skip-tls-verify")
			assert.Contains(t, showFlags, "version")
		})
	}
}

func TestMCPCommands(t *testing.T) {
	root, err := newRootCmdWithConfig(action.NewConfiguration(), &bytes.Buffer{}, nil, SetupLogging)
	require.NoError(t, err)

	mcpCmd, _, err := root.Find([]string{"mcp"})
	require.NoError(t, err)
	require.Equal(t, "mcp", mcpCmd.Name())

	var names []string
	for _, command := range mcpCmd.Commands() {
		names = append(names, command.Name())
		assert.NotNil(t, command.InheritedFlags().Lookup("allow-write"), command.Name())
	}
	assert.ElementsMatch(t, []string{"start", "stream", "tools"}, names)

	for _, name := range []string{"claude", "vscode", "cursor"} {
		found, remaining, err := root.Find([]string{"mcp", name, "enable"})
		require.NoError(t, err)
		assert.Same(t, mcpCmd, found, name)
		assert.Equal(t, []string{name, "enable"}, remaining)
	}
}

func TestValidateMCPArgs(t *testing.T) {
	for _, args := range [][]string{{"--password=secret"}, {"chart", "--plain-http"}, {"-"}} {
		_, _, err := validateMCPArgs(context.Background(), nil, ophis.ToolInput{Args: args}, func(context.Context, *mcp.CallToolRequest, ophis.ToolInput) (*mcp.CallToolResult, ophis.ToolOutput, error) {
			t.Fatal("must not execute with option-like positional arguments")
			return nil, ophis.ToolOutput{}, nil
		})
		require.ErrorContains(t, err, "must not start with '-'")
	}

	called := false
	_, _, err := validateMCPArgs(context.Background(), nil, ophis.ToolInput{Args: []string{"my-chart"}}, func(context.Context, *mcp.CallToolRequest, ophis.ToolInput) (*mcp.CallToolResult, ophis.ToolOutput, error) {
		called = true
		return nil, ophis.ToolOutput{}, nil
	})
	require.NoError(t, err)
	assert.True(t, called)
}
