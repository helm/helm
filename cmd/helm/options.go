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

package main

import (
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/cmd/helm/require"
	"io"
	"k8s.io/kubectl/pkg/util/templates"
)

// NewCmdOptions implements the options command
func newCmdOptions(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "options",
		DisableFlagsInUseLine: true,
		Short:                 "print the list of flags inherited by all commands",
		Long:                  "print the list of flags inherited by all commands",
		Args:                  require.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	// The `options` command needs to write its output to the `out` stream
	// (typically stdout). Without calling SetOutput here, the Usage()
	// function call will fall back to stderr.
	//
	// See https://github.com/kubernetes/kubernetes/pull/46394 for details.
	cmd.SetOutput(out)

	templates.UseOptionsTemplates(cmd)
	return cmd
}
