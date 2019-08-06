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
	"fmt"
	"io"

	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
)

func newRepoListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list chart repositories",
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := repo.LoadFile(helmpath.RepositoryFile())
			if err != nil {
				return err
			}
			if len(f.Repositories) == 0 {
				return errors.New("no repositories to show")
			}
			table := uitable.New()
			table.AddRow("NAME", "URL")
			for _, re := range f.Repositories {
				table.AddRow(re.Name, re.URL)
			}
			fmt.Fprintln(out, table)
			return nil
		},
	}

	return cmd
}
