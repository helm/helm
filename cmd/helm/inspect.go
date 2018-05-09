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
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi/chart"
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
	verify    bool
	keyring   string
	version   string
	repoURL   string
	username  string
	password  string
	certFile  string
	keyFile   string
	caFile    string
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(o.repoURL, o.username, o.password, args[0], o.version, o.verify, o.keyring,
				o.certFile, o.keyFile, o.caFile)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = valuesOnly
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(o.repoURL, o.username, o.password, args[0], o.version, o.verify, o.keyring,
				o.certFile, o.keyFile, o.caFile)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = chartOnly
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(o.repoURL, o.username, o.password, args[0], o.version, o.verify, o.keyring,
				o.certFile, o.keyFile, o.caFile)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			o.output = readmeOnly
			if err := checkArgsLength(len(args), "chart name"); err != nil {
				return err
			}
			cp, err := locateChartPath(o.repoURL, o.username, o.password, args[0], o.version, o.verify, o.keyring,
				o.certFile, o.keyFile, o.caFile)
			if err != nil {
				return err
			}
			o.chartpath = cp
			return o.run(out)
		},
	}

	cmds := []*cobra.Command{inspectCommand, readmeSubCmd, valuesSubCmd, chartSubCmd}
	vflag := "verify"
	vdesc := "verify the provenance data for this chart"
	for _, subCmd := range cmds {
		subCmd.Flags().BoolVar(&o.verify, vflag, false, vdesc)
	}

	kflag := "keyring"
	kdesc := "path to the keyring containing public verification keys"
	kdefault := defaultKeyring()
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.keyring, kflag, kdefault, kdesc)
	}

	verflag := "version"
	verdesc := "version of the chart. By default, the newest chart is shown"
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.version, verflag, "", verdesc)
	}

	repoURL := "repo"
	repoURLdesc := "chart repository url where to locate the requested chart"
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.repoURL, repoURL, "", repoURLdesc)
	}

	username := "username"
	usernamedesc := "chart repository username where to locate the requested chart"
	inspectCommand.Flags().StringVar(&o.username, username, "", usernamedesc)
	valuesSubCmd.Flags().StringVar(&o.username, username, "", usernamedesc)
	chartSubCmd.Flags().StringVar(&o.username, username, "", usernamedesc)

	password := "password"
	passworddesc := "chart repository password where to locate the requested chart"
	inspectCommand.Flags().StringVar(&o.password, password, "", passworddesc)
	valuesSubCmd.Flags().StringVar(&o.password, password, "", passworddesc)
	chartSubCmd.Flags().StringVar(&o.password, password, "", passworddesc)

	certFile := "cert-file"
	certFiledesc := "verify certificates of HTTPS-enabled servers using this CA bundle"
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.certFile, certFile, "", certFiledesc)
	}

	keyFile := "key-file"
	keyFiledesc := "identify HTTPS client using this SSL key file"
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.keyFile, keyFile, "", keyFiledesc)
	}

	caFile := "ca-file"
	caFiledesc := "chart repository url where to locate the requested chart"
	for _, subCmd := range cmds {
		subCmd.Flags().StringVar(&o.caFile, caFile, "", caFiledesc)
	}

	for _, subCmd := range cmds[1:] {
		inspectCommand.AddCommand(subCmd)
	}

	return inspectCommand
}

func (i *inspectOptions) run(out io.Writer) error {
	chrt, err := chartutil.Load(i.chartpath)
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
