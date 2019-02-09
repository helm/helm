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

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
)

const pullDesc = `
Retrieve a package from a package repository, and download it locally.

This is useful for fetching packages to inspect, modify, or repackage. It can
also be used to perform cryptographic verification of a chart without installing
the chart.

There are options for unpacking the chart after download. This will create a
directory for the chart and uncompress into that directory.

If the --verify flag is specified, the requested chart MUST have a provenance
file, and MUST pass the verification process. Failure in any part of this will
result in an error, and the chart will not be saved locally.
`

func newPullCmd(out io.Writer) *cobra.Command {
	client := action.NewPull()
	client.Out = out

	cmd := &cobra.Command{
		Use:     "pull [chart URL | repo/chartname] [...]",
		Short:   "download a chart from a repository and (optionally) unpack it in local directory",
		Aliases: []string{"fetch"},
		Long:    pullDesc,
		Args:    require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Settings = settings
			if client.Version == "" && client.Devel {
				debug("setting version to >0.0.0-0")
				client.Version = ">0.0.0-0"
			}

			for i := 0; i < len(args); i++ {
				if err := client.Run(args[i]); err != nil {
					return err
				}
			}
			return nil
		},
	}

	client.AddFlags(cmd.Flags())

	return cmd
}
