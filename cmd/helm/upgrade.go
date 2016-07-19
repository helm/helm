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
	"io/ioutil"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

const upgradeDesc = `
This command upgrades a release to a new version of a chart.

The upgrade arguments must be a release and a chart. The chart
argument can be a relative path to a packaged or unpackaged chart.
`

type upgradeCmd struct {
	release    string
	chart      string
	out        io.Writer
	client     helm.Interface
	dryRun     bool
	valuesFile string
}

func newUpgradeCmd(client helm.Interface, out io.Writer) *cobra.Command {

	upgrade := &upgradeCmd{
		out:    out,
		client: client,
	}

	cmd := &cobra.Command{
		Use:               "upgrade [RELEASE] [CHART]",
		Short:             "upgrade a release",
		Long:              upgradeDesc,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(2, len(args), "release name, chart path"); err != nil {
				return err
			}

			upgrade.release = args[0]
			upgrade.chart = args[1]
			upgrade.client = ensureHelmClient(upgrade.client)

			return upgrade.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&upgrade.valuesFile, "values", "f", "", "path to a values YAML file")
	f.BoolVar(&upgrade.dryRun, "dry-run", false, "simulate an upgrade")

	return cmd
}

func (u *upgradeCmd) run() error {
	chartPath, err := locateChartPath(u.chart)
	if err != nil {
		return err
	}

	rawVals := []byte{}
	if u.valuesFile != "" {
		rawVals, err = ioutil.ReadFile(u.valuesFile)
		if err != nil {
			return err
		}
	}

	_, err = u.client.UpdateRelease(u.release, chartPath, helm.UpdateValueOverrides(rawVals), helm.UpgradeDryRun(u.dryRun))
	if err != nil {
		return prettyError(err)
	}

	fmt.Fprintf(u.out, "It's not you. It's me\nYour upgrade looks valid but this command is still under active development.\nHang tight.\n")

	return nil

}
