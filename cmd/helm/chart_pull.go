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
	"helm.sh/helm/v3/pkg/action"
)

const chartPullDesc = `
Download a chart from a remote registry.

This will store the chart in the local registry cache to be used later.
`

func newChartPullCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var insecureOpt, plainHTTPOpt bool
	cmd := &cobra.Command{
		Use:    "pull [ref]",
		Short:  "pull a chart from remote",
		Long:   chartPullDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			return action.NewChartPull(cfg).Run(out, ref, insecureOpt, plainHTTPOpt)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&insecureOpt, "insecure", "", false, "allow connections to TLS registry without certs")
	f.BoolVarP(&plainHTTPOpt, "plain-http", "", false, "use plain http and not https")

	return cmd
}
