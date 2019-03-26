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
	"io"

	"github.com/spf13/cobra"
)

const pluginHelp = `
Provides utilities for interacting with plugins.

Plugins provide extended functionality that is not part of Helm. Please refer to the documentation
and examples for more information about how write your own plugins.
`

func newPluginCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "utilities for interacting with Helm plugins",
		Long:  pluginHelp,
	}
	cmd.AddCommand(
		newPluginListCmd(out),
	)
	return cmd
}
