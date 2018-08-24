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
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
)

const inspectDesc = `
This command inspects a chart and displays information. It takes a chart reference
('stable/drupal'), a full path to a directory or packaged chart, or a URL.

Inspect prints the contents of the Chart.yaml file and the values.yaml file.
`

const inspectValuesDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the values.yaml file
`

const inspectChartDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the Charts.yaml file
`

const readmeChartDesc = `
This command inspects a chart (directory, file, or URL) and displays the contents
of the README file
`

type inspectOptions struct {
	chartpath string
	output    string

	chartPathOptions
}

const (
	chartOnly  = "chart"
	valuesOnly = "values"
	readmeOnly = "readme"
	all        = "all"
)

var readmeFileNames = []string{"readme.md", "readme.txt", "readme"}

func newInspectCmd(out io.Writer) *cobra.Command {
	o := &inspectOptions{output: all}

	inspectCommand := &cobra.Command{
		Use:   "inspect [CHART]",
		Short: "inspect a chart",
		Long:  inspectDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cp, err := o.locateChart(args[0])
			if err != nil {
				return err
			}
			o.chartpath = cp
			return o.run(out)
		},
	}

	valuesSubCmd := &cobra.Command{
		Use:   "values [CHART]",
		Short: "shows inspect values",
		Long:  inspectValuesDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = valuesOnly
			cp, err := o.locateChart(args[0])
			if err != nil {
				return err
			}
			o.chartpath = cp
			return o.run(out)
		},
	}

	chartSubCmd := &cobra.Command{
		Use:   "chart [CHART]",
		Short: "shows inspect chart",
		Long:  inspectChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = chartOnly
			cp, err := o.locateChart(args[0])
			if err != nil {
				return err
			}
			o.chartpath = cp
			return o.run(out)
		},
	}

	readmeSubCmd := &cobra.Command{
		Use:   "readme [CHART]",
		Short: "shows inspect readme",
		Long:  readmeChartDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = readmeOnly
			cp, err := o.locateChart(args[0])
			if err != nil {
				return err
			}
			o.chartpath = cp
			return o.run(out)
		},
	}

	cmds := []*cobra.Command{inspectCommand, readmeSubCmd, valuesSubCmd, chartSubCmd}
	for _, subCmd := range cmds {
		o.chartPathOptions.addFlags(subCmd.Flags())
	}

	for _, subCmd := range cmds[1:] {
		inspectCommand.AddCommand(subCmd)
	}

	return inspectCommand
}

func (i *inspectOptions) run(out io.Writer) error {
	chrt, err := loader.Load(i.chartpath)
	if err != nil {
		return err
	}
	cf, err := yaml.Marshal(chrt.Metadata)
	if err != nil {
		return err
	}

	if i.output == chartOnly || i.output == all {
		fmt.Fprintln(out, string(cf))
	}

	if (i.output == valuesOnly || i.output == all) && chrt.Values != nil {
		if i.output == all {
			fmt.Fprintln(out, "---")
		}
		fmt.Fprintln(out, string(chrt.Values))
	}

	if i.output == readmeOnly || i.output == all {
		if i.output == all {
			fmt.Fprintln(out, "---")
		}
		readme := findReadme(chrt.Files)
		if readme == nil {
			return nil
		}
		fmt.Fprintln(out, string(readme.Data))
	}
	return nil
}

func findReadme(files []*chart.File) (file *chart.File) {
	for _, file := range files {
		for _, n := range readmeFileNames {
			if strings.EqualFold(file.Name, n) {
				return file
			}
		}
	}
	return nil
}
