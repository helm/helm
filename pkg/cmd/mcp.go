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

// MCP command lists are exact allowlists. New Helm commands must not
// automatically become available to MCP clients.
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

// These commands change releases or execute test hooks in a Kubernetes cluster.
var mcpWriteCommands = []string{
	"helm install",
	"helm upgrade",
	"helm rollback",
	"helm uninstall",
	"helm test",
}

func newMCPCmd(root *cobra.Command) *cobra.Command {
	// Only explicitly listed commands are exposed, including when Helm gains new
	// commands in the future. Mark writes as destructive for MCP clients.
	for _, group := range []struct {
		paths       []string
		destructive bool
	}{
		{mcpReadCommands, false},
		{mcpWriteCommands, true},
	} {
		for _, path := range group.paths {
			parts := strings.Fields(path)[1:]
			command, _, err := root.Find(parts)
			if err != nil || command.CommandPath() != path {
				continue
			}
			if command.Annotations == nil {
				command.Annotations = make(map[string]string)
			}
			if group.destructive {
				command.Annotations[ophis.AnnotationReadOnly] = "false"
				command.Annotations[ophis.AnnotationDestructive] = "true"
			} else {
				command.Annotations[ophis.AnnotationReadOnly] = "true"
			}
		}
	}

	selector := ophis.Selector{
		CmdSelector: ophis.AllowCmds(mcpReadCommands...),
		// Do not accept credentials or insecure transport options as MCP input.
		LocalFlagSelector: ophis.ExcludeFlags(
			"username", "password", "cert-file", "key-file",
			"insecure-skip-tls-verify", "plain-http", "pass-credentials",
			// A post-renderer runs an arbitrary local executable.
			"post-renderer", "post-renderer-args",
		),
		// Helm has many global authentication, impersonation and debug flags.
		InheritedFlagSelector: ophis.AllowFlags("namespace", "kube-context", "kubeconfig"),
		Middleware:            validateMCPArgs,
	}
	writeSelector := selector
	writeSelector.CmdSelector = ophis.AllowCmds(mcpWriteCommands...)
	config := &ophis.Config{Selectors: []ophis.Selector{selector}}
	mcpCmd := ophis.Command(config)
	var allowWrite bool
	mcpCmd.PersistentFlags().BoolVar(&allowWrite, "allow-write", false, "also expose commands that change releases or run test hooks")
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
			continue
		}
		command.PreRun = func(_ *cobra.Command, _ []string) {
			config.Selectors = []ophis.Selector{selector}
			if allowWrite {
				config.Selectors = append(config.Selectors, writeSelector)
			}
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
