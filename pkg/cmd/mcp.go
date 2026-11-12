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
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

// mcpReadCommands is intentionally an exact allowlist. Adding a new Helm command
// must not silently grant an MCP client the ability to execute it.
var mcpReadCommands = []string{
	"helm list",
	"helm status",
	"helm history",
	"helm get metadata",
	"helm show chart",
	"helm show values",
	"helm show readme",
	"helm show crds",
	"helm search repo",
	"helm search hub",
	"helm repo list",
	"helm version",
}

func newMCPCmd(root *cobra.Command) *cobra.Command {
	allowed := ophis.AllowCmds(mcpReadCommands...)
	// Mark only the selected commands read-only in the generated MCP metadata.
	for _, path := range mcpReadCommands {
		parts := strings.Fields(path)[1:]
		command, _, err := root.Find(parts)
		if err != nil || command.CommandPath() != path {
			continue
		}
		if command.Annotations == nil {
			command.Annotations = make(map[string]string)
		}
		command.Annotations[ophis.AnnotationReadOnly] = "true"
	}

	mcpCmd := ophis.Command(&ophis.Config{
		Selectors: []ophis.Selector{{
			CmdSelector: allowed,
			// Do not accept credentials or insecure transport options as MCP input.
			LocalFlagSelector: ophis.ExcludeFlags(
				"username", "password", "cert-file", "key-file",
				"insecure-skip-tls-verify", "plain-http", "pass-credentials",
			),
			// Helm has many global authentication, impersonation and debug flags.
			InheritedFlagSelector: ophis.AllowFlags("namespace", "kube-context", "kubeconfig"),
			Middleware:            validateMCPArgs,
		}},
	})
	mcpCmd.Long = "Manage the Helm MCP server"
	// Ophis also provides editor-specific configuration commands. Helm only
	// exposes generic server management commands; clients configure themselves.
	for _, command := range mcpCmd.Commands() {
		switch command.Name() {
		case "start":
			command.Long = "Start the Helm MCP server over standard input and output"
		case "stream", "tools":
		default:
			mcpCmd.RemoveCommand(command)
		}
	}
	return mcpCmd
}

// Ophis appends positional arguments after flags when spawning the Helm CLI.
// Reject option-looking arguments so they cannot smuggle flags omitted from
// the tool schema (including credentials or insecure transport settings).
func validateMCPArgs(ctx context.Context, req *mcp.CallToolRequest, input ophis.ToolInput, next ophis.ExecuteFunc) (*mcp.CallToolResult, ophis.ToolOutput, error) {
	for _, arg := range input.Args {
		if strings.HasPrefix(arg, "-") {
			return nil, ophis.ToolOutput{}, fmt.Errorf("MCP positional argument %q must not start with '-'", arg)
		}
	}
	return next(ctx, req, input)
}
