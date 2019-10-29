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

	"helm.sh/helm/v3/pkg/action"
)

const chartHelp = `
This command consists of multiple subcommands to work with the chart cache.

The subcommands can be used to push, pull, tag, list, or remove Helm charts.
`

func newChartCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "chart",
		Short:             "push, pull, tag, or remove Helm charts",
		Long:              chartHelp,
		Hidden:            !FeatureGateOCI.IsEnabled(),
		PersistentPreRunE: checkOCIFeatureGate(),
	}
	cmd.AddCommand(
		newChartListCmd(cfg, out),
		newChartExportCmd(cfg, out),
		newChartPullCmd(cfg, out),
		newChartPushCmd(cfg, out),
		newChartRemoveCmd(cfg, out),
		newChartSaveCmd(cfg, out),
	)
	return cmd
}
