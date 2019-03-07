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

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/helm"
)

const releaseTestResultsDesc = `
This command prints the results of the last test run execution for a
given release.

Example usage:
    $ helm test results: [RELEASE]
`

type releaseTestResultsOptions struct {
	name   string
	client helm.Interface
}

func newReleaseTestResultsCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &releaseTestResultsOptions{client: c}

	cmd := &cobra.Command{
		Use:   "results [RELEASE]",
		Short: "show latest test results for a release",
		Long:  releaseTestResultsDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.name = args[0]
			o.client = ensureHelmClient(o.client, false)
			fmt.Fprintf(out, "NOT IMPLEMENTED YET")
			return nil
		},
	}

	return cmd
}
