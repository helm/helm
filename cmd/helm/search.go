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

const searchDesc = `
Search provides the ability to search for Helm charts in the various places
they can be stored including the Artifact Hub and repositories you have added.
Use search subcommands to search different locations for charts.
`

func newSearchCmd(out io.Writer) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "search [keyword]",
		Short: "search for a keyword in charts",
		Long:  searchDesc,
	}

	cmd.AddCommand(newSearchHubCmd(out))
	cmd.AddCommand(newSearchRepoCmd(out))

	return cmd
}
