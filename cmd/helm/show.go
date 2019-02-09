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

const showDesc = `
This command inspects a chart and displays information. It takes a chart reference
('stable/drupal'), a full path to a directory or packaged chart, or a URL.

Inspect prints the contents of the Chart.yaml file and the values.yaml file.
`

const showValuesDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the values.yaml file
`

const showChartDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the Charts.yaml file
`

const readmeChartDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the README file
`

func newShowCmd(out io.Writer) *cobra.Command {
	client := action.NewShow(out, action.ShowAll)

	showCommand := &cobra.Command{
		Use:     "show [CHART]",
		Short:   "inspect a chart",
		Aliases: []string{"inspect"},
		Long:    showDesc,
		Args:    require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cp, err := client.ChartPathOptions.LocateChart(args[0], settings)
			if err != nil {
				return err
			}
			return client.Run(cp)
		},
	}

	valuesSubCmd := &cobra.Command{
		Use:   "values [CHART]",
		Short: "shows values for this chart",
		Long:  showValuesDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowValues
			cp, err := client.ChartPathOptions.LocateChart(args[0], settings)
			if err != nil {
				return err
			}
			return client.Run(cp)
		},
	}

	chartSubCmd := &cobra.Command{
		Use:   "chart [CHART]",
		Short: "shows the chart",
		Long:  showChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowChart
			cp, err := client.ChartPathOptions.LocateChart(args[0], settings)
			if err != nil {
				return err
			}
			return client.Run(cp)
		},
	}

	readmeSubCmd := &cobra.Command{
		Use:   "readme [CHART]",
		Short: "shows the chart's README",
		Long:  readmeChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowReadme
			cp, err := client.ChartPathOptions.LocateChart(args[0], settings)
			if err != nil {
				return err
			}
			return client.Run(cp)
		},
	}

	cmds := []*cobra.Command{showCommand, readmeSubCmd, valuesSubCmd, chartSubCmd}
	for _, subCmd := range cmds {
		client.AddFlags(subCmd.Flags())
	}

	for _, subCmd := range cmds[1:] {
		showCommand.AddCommand(subCmd)
	}

	return showCommand
}
