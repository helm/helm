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

	"helm.sh/helm/v3/cmd/helm/require"
)

type repoImportOptions struct {
	filePath string
}

func newRepoImportCmd(out io.Writer) *cobra.Command {
	o := &repoImportOptions{}

	cmd := &cobra.Command{
		Use:               "import [NAME]",
		Aliases:           []string{"im"},
		Short:             "import chart repositories",
		Args:              require.ExactArgs(1),
		ValidArgsFunction: noCompletions, // FIXME
		RunE: func(cmd *cobra.Command, args []string) error {
			o.filePath = args[0]
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.filePath, "file", "", "path to the file to import")

	return cmd
}

func (o *repoImportOptions) run(out io.Writer) error {
	out.Write([]byte("repo import called\n"))
	return nil
}
