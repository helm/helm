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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayp/ophis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
)

func TestMCPTools(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	root, err := newRootCmdWithConfig(action.NewConfiguration(), &out, []string{"mcp", "tools"}, SetupLogging)
	require.NoError(t, err)
	root.SetArgs([]string{"mcp", "tools"})
	require.NoError(t, root.Execute())

	data, err := os.ReadFile("mcp-tools.json")
	require.NoError(t, err)
	var tools []struct {
		Name        string `json:"name"`
		Annotations struct {
			ReadOnlyHint bool `json:"readOnlyHint"`
		} `json:"annotations"`
		InputSchema struct {
			Properties map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	require.NoError(t, json.Unmarshal(data, &tools))
	require.Len(t, tools, len(mcpReadCommands))

	for _, tool := range tools {
		path := strings.ReplaceAll(tool.Name, "_", " ")
		assert.Contains(t, mcpReadCommands, path)
		assert.True(t, tool.Annotations.ReadOnlyHint, tool.Name)
		flags := tool.InputSchema.Properties["flags"].Properties
		assert.NotContains(t, flags, "kube-token", tool.Name)
		assert.NotContains(t, flags, "kube-as-user", tool.Name)
		assert.NotContains(t, flags, "debug", tool.Name)
		assert.Contains(t, flags, "namespace", tool.Name)
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
