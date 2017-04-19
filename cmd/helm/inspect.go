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

type inspectCmd struct {
	chartpath string
	output    string
	verify    bool
	keyring   string
	out       io.Writer
	version   string
	repoURL   string

	certFile string
	keyFile  string
	caFile   string
}

const (
	chartOnly  = "chart"
	valuesOnly = "values"
	both       = "both"
)

func newInspectCmd(out io.Writer) *cobra.Command {
	insp := &inspectCmd{
		out:    out,
		output: both,
	}

	inspectCommand := &cobra.Command{
		Use:   "inspect [CHART]",
		Short: "inspect a chart",
		Long:  inspectDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(insp.repoURL, args[0], insp.version, insp.verify, insp.keyring,
				insp.certFile, insp.keyFile, insp.caFile)
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	valuesSubCmd := &cobra.Command{
		Use:   "values [CHART]",
		Short: "shows inspect values",
		Long:  inspectValuesDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			insp.output = valuesOnly
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(insp.repoURL, args[0], insp.version, insp.verify, insp.keyring,
				insp.certFile, insp.keyFile, insp.caFile)
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	chartSubCmd := &cobra.Command{
		Use:   "chart [CHART]",
		Short: "shows inspect chart",
		Long:  inspectChartDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			insp.output = chartOnly
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(insp.repoURL, args[0], insp.version, insp.verify, insp.keyring,
				insp.certFile, insp.keyFile, insp.caFile)
			if err != nil {
				return err
			}
			insp.chartpath = cp
			return insp.run()
		},
	}

	vflag := "verify"
	vdesc := "verify the provenance data for this chart"
	inspectCommand.Flags().BoolVar(&insp.verify, vflag, false, vdesc)
	valuesSubCmd.Flags().BoolVar(&insp.verify, vflag, false, vdesc)
	chartSubCmd.Flags().BoolVar(&insp.verify, vflag, false, vdesc)

	kflag := "keyring"
	kdesc := "path to the keyring containing public verification keys"
	kdefault := defaultKeyring()
	inspectCommand.Flags().StringVar(&insp.keyring, kflag, kdefault, kdesc)
	valuesSubCmd.Flags().StringVar(&insp.keyring, kflag, kdefault, kdesc)
	chartSubCmd.Flags().StringVar(&insp.keyring, kflag, kdefault, kdesc)

	verflag := "version"
	verdesc := "version of the chart. By default, the newest chart is shown"
	inspectCommand.Flags().StringVar(&insp.version, verflag, "", verdesc)
	valuesSubCmd.Flags().StringVar(&insp.version, verflag, "", verdesc)
	chartSubCmd.Flags().StringVar(&insp.version, verflag, "", verdesc)

	repoURL := "repo"
	repoURLdesc := "chart repository url where to locate the requested chart"
	inspectCommand.Flags().StringVar(&insp.repoURL, repoURL, "", repoURLdesc)
	valuesSubCmd.Flags().StringVar(&insp.repoURL, repoURL, "", repoURLdesc)
	chartSubCmd.Flags().StringVar(&insp.repoURL, repoURL, "", repoURLdesc)

	certFile := "cert-file"
	certFiledesc := "verify certificates of HTTPS-enabled servers using this CA bundle"
	inspectCommand.Flags().StringVar(&insp.certFile, certFile, "", certFiledesc)
	valuesSubCmd.Flags().StringVar(&insp.certFile, certFile, "", certFiledesc)
	chartSubCmd.Flags().StringVar(&insp.certFile, certFile, "", certFiledesc)

	keyFile := "key-file"
	keyFiledesc := "identify HTTPS client using this SSL key file"
	inspectCommand.Flags().StringVar(&insp.keyFile, keyFile, "", keyFiledesc)
	valuesSubCmd.Flags().StringVar(&insp.keyFile, keyFile, "", keyFiledesc)
	chartSubCmd.Flags().StringVar(&insp.keyFile, keyFile, "", keyFiledesc)

	caFile := "ca-file"
	caFiledesc := "chart repository url where to locate the requested chart"
	inspectCommand.Flags().StringVar(&insp.caFile, caFile, "", caFiledesc)
	valuesSubCmd.Flags().StringVar(&insp.caFile, caFile, "", caFiledesc)
	chartSubCmd.Flags().StringVar(&insp.caFile, caFile, "", caFiledesc)

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

	if (i.output == valuesOnly || i.output == both) && chrt.Values != nil {
		if i.output == both {
			fmt.Fprintln(i.out, "---")
		}
		fmt.Fprintln(i.out, chrt.Values.Raw)
	}

	return nil
}
