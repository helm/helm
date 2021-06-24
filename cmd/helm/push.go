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

	"helm.sh/helm/v3/cmd/helm/require"
	experimental "helm.sh/helm/v3/internal/experimental/action"
	"helm.sh/helm/v3/pkg/action"
)

const pushDesc = `
Upload a chart to a registry.

If the chart has an associated provenance file,
it will also be uploaded.
`

func newPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := experimental.NewPushWithOpts(experimental.WithPushConfig(cfg))

	cmd := &cobra.Command{
		Use:               "push [chart] [remote]",
		Short:             "push a chart to remote",
		Long:              pushDesc,
		Hidden:            !FeatureGateOCI.IsEnabled(),
		PersistentPreRunE: checkOCIFeatureGate(),
		Args:              require.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartRef := args[0]
			remote := args[1]
			client.Settings = settings
			output, err := client.Run(chartRef, remote)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	return cmd
}
