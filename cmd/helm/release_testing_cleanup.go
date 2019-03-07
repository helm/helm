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

const releaseTestCleanupDesc = `
This command deletes the all test pods for the given release. The argument
this command takes is the name of a deployed release.

Example usage:
     $ helm test cleanup [RELEASE]
`

type releaseTestCleanupOptions struct {
	name   string
	client helm.Interface
}

func newReleaseTestCleanupCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &releaseTestCleanupOptions{client: c}

	cmd := &cobra.Command{
		Use:   "cleanup [RELEASE]",
		Short: "delete test pods for a release",
		Long:  releaseTestCleanupDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.name = args[0]
			o.client = ensureHelmClient(o.client, false)
			fmt.Fprintf(out, "NOT IMPLEMENTED YET\n")
			return nil
		},
	}

	return cmd
}
