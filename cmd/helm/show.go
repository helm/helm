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
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
)

const showDesc = `
This command consists of multiple subcommands to display information about a chart
`

const showAllDesc = `
This command inspects a chart (directory, file, or URL) and displays all its content
(values.yaml, Charts.yaml, README)
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
	client := action.NewShow(action.ShowAll)

	showCommand := &cobra.Command{
		Use:     "show",
		Short:   "show information of a chart",
		Aliases: []string{"inspect"},
		Long:    showDesc,
		Args:    require.NoArgs,
	}

	// Function providing dynamic auto-completion
	validArgsFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) != 0 {
			return nil, completion.BashCompDirectiveNoFileComp
		}
		return compListCharts(toComplete, true)
	}

	all := &cobra.Command{
		Use:   "all [CHART]",
		Short: "shows all information of the chart",
		Long:  showAllDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowAll
			output, err := runShow(args, client)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	valuesSubCmd := &cobra.Command{
		Use:   "values [CHART]",
		Short: "shows the chart's values",
		Long:  showValuesDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowValues
			output, err := runShow(args, client)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	chartSubCmd := &cobra.Command{
		Use:   "chart [CHART]",
		Short: "shows the chart's definition",
		Long:  showChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowChart
			output, err := runShow(args, client)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	readmeSubCmd := &cobra.Command{
		Use:   "readme [CHART]",
		Short: "shows the chart's README",
		Long:  readmeChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client.OutputFormat = action.ShowReadme
			output, err := runShow(args, client)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	cmds := []*cobra.Command{all, readmeSubCmd, valuesSubCmd, chartSubCmd}
	for _, subCmd := range cmds {
		addShowFlags(showCommand, subCmd, client)

		// Register the completion function for each subcommand
		completion.RegisterValidArgsFunc(subCmd, validArgsFunc)
	}

	return showCommand
}

func addShowFlags(showCmd *cobra.Command, subCmd *cobra.Command, client *action.Show) {
	f := subCmd.Flags()

	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	addChartPathOptionsFlags(f, &client.ChartPathOptions)
	showCmd.AddCommand(subCmd)
}

func runShow(args []string, client *action.Show) (string, error) {
	debug("Original chart version: %q", client.Version)
	if client.Version == "" && client.Devel {
		debug("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	cp, err := client.ChartPathOptions.LocateChart(args[0], settings)
	if err != nil {
		return "", err
	}
	return client.Run(cp)
}
