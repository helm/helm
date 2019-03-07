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

	"k8s.io/helm/pkg/helm"
)

const releaseTestDesc = `
This command consists of multiple subcommands to run and manage tests
performed on a release.

For now, it can be used to run tests for a given release. More
subcommands will be added in future iterations of this command.

Example usage:
     $ helm test run [RELEASE]

`

func newReleaseTestCmd(c helm.Interface, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "run and manage tests on a release",
		Long:  releaseTestDesc,
	}
	cmd.AddCommand(
		newReleaseTestRunCmd(c, out),
		newReleaseTestCleanupCmd(c, out),
		newReleaseTestResultsCmd(c, out),
	)

	return cmd
}
