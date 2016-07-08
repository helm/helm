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

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

const upgradeDesc = `
This command upgrades a release to a new version of a chart.

The upgrade arguments must be a release and a chart. The chart
argument can be a relative path to a packaged or unpackaged chart.
`

var upgradeCmd = &cobra.Command{
	Use:               "upgrade [RELEASE] [CHART]",
	Short:             "upgrade a release",
	Long:              upgradeDesc,
	RunE:              runUpgrade,
	PersistentPreRunE: setupConnection,
}

// upgrade flags
var (
	// upgradeDryRun performs a dry-run upgrade
	upgradeDryRun bool
	// upgradeValues is the filename of supplied values.
	upgradeValues string
)

func init() {
	f := upgradeCmd.Flags()
	f.StringVarP(&upgradeValues, "values", "f", "", "path to a values YAML file")
	f.BoolVar(&upgradeDryRun, "dry-run", false, "simulate an upgrade")

	RootCommand.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(2, len(args), "release name, chart path"); err != nil {
		return err
	}

	chartPath, err := locateChartPath(args[1])
	if err != nil {
		return err
	}

	rawVals, err := vals(upgradeValues)
	if err != nil {
		return err
	}

	_, err = helm.UpdateRelease(args[0], chartPath, rawVals, upgradeDryRun)
	if err != nil {
		return prettyError(err)
	}

	fmt.Println("Coming SOON to a Helm near YOU!")

	return nil
}
