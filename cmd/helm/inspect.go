/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
)

const inspectDesc = `
This command inspects a chart (directory, file, or URL) and displays information.

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

type inspectCmd struct {
	chartpath string
	output    string
	out       io.Writer
	client    helm.Interface
}

const (
	chartOnly  = "chart"
	valuesOnly = "values"
	both       = "both"
)

func newInspectCmd(c helm.Interface, out io.Writer) *cobra.Command {
	insp := &inspectCmd{
		client: c,
		out:    out,
		output: both,
	}

	inspectCommand := &cobra.Command{
		Use:   "inspect [CHART]",
		Short: "inspect a chart",
		Long:  inspectDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(1, len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(args[0])
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	valuesSubCmd := &cobra.Command{
		Use:   "values",
		Short: "shows inspect values",
		Long:  inspectValuesDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			insp.output = valuesOnly
			cp, err := locateChartPath(args[0])
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	chartSubCmd := &cobra.Command{
		Use:   "chart",
		Short: "shows inspect chart",
		Long:  inspectChartDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			insp.output = chartOnly
			cp, err := locateChartPath(args[0])
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	inspectCommand.AddCommand(valuesSubCmd)
	inspectCommand.AddCommand(chartSubCmd)

	return inspectCommand
}

func (i *inspectCmd) run() error {
	chrt, err := chartutil.Load(i.chartpath)
	if err != nil {
		return err
	}
	cf, err := yaml.Marshal(chrt.Metadata)
	if err != nil {
		return err
	}

	if i.output == chartOnly || i.output == both {
		fmt.Fprintln(i.out, string(cf))
	}

	if i.output == both {
		fmt.Fprintln(i.out, "---")
	}

	if i.output == valuesOnly || i.output == both {
		fmt.Fprintln(i.out, chrt.Values.Raw)
	}

	return nil
}
