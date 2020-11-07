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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
)

var repoHelm = `
This command consists of multiple subcommands to interact with chart repositories.

It can be used to add, remove, list, and index chart repositories.
`

func newRepoCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo add|remove|list|index|update [ARGS]",
		Short: "add, list, remove, update, and index chart repositories",
		Long:  repoHelm,
		Args:  require.NoArgs,
	}

	cmd.AddCommand(newRepoAddCmd(out))
	cmd.AddCommand(newRepoListCmd(out))
	cmd.AddCommand(newRepoRemoveCmd(out))
	cmd.AddCommand(newRepoIndexCmd(out))
	cmd.AddCommand(newRepoUpdateCmd(out))

	return cmd
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}
