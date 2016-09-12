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
	"bytes"
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

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line.
`

type upgradeCmd struct {
	release      string
	chart        string
	out          io.Writer
	client       helm.Interface
	dryRun       bool
	disableHooks bool
	valuesFile   string
	values       *values
	verify       bool
	keyring      string
}

func newUpgradeCmd(client helm.Interface, out io.Writer) *cobra.Command {

	upgrade := &upgradeCmd{
		out:    out,
		client: client,
		values: new(values),
	}

	cmd := &cobra.Command{
		Use:               "upgrade [RELEASE] [CHART]",
		Short:             "upgrade a release",
		Long:              upgradeDesc,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name", "chart path"); err != nil {
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
	f.Var(upgrade.values, "set", "set values on the command line. Separate values with commas: key1=val1,key2=val2")
	f.BoolVar(&upgrade.disableHooks, "disable-hooks", false, "disable pre/post upgrade hooks")
	f.BoolVar(&upgrade.verify, "verify", false, "verify the provenance of the chart before upgrading")
	f.StringVar(&upgrade.keyring, "keyring", defaultKeyring(), "the path to the keyring that contains public singing keys")

	return cmd
}

func (u *upgradeCmd) vals() ([]byte, error) {
	var buffer bytes.Buffer

	// User specified a values file via -f/--values
	if u.valuesFile != "" {
		bytes, err := ioutil.ReadFile(u.valuesFile)
		if err != nil {
			return []byte{}, err
		}
		buffer.Write(bytes)
	}

	// User specified value pairs via --set
	// These override any values in the specified file
	if len(u.values.pairs) > 0 {
		bytes, err := u.values.yaml()
		if err != nil {
			return []byte{}, err
		}
		buffer.Write(bytes)
	}

	return buffer.Bytes(), nil
}

func (u *upgradeCmd) run() error {
	chartPath, err := locateChartPath(u.chart, u.verify, u.keyring)
	if err != nil {
		return err
	}

	rawVals, err := u.vals()
	if err != nil {
		return err
	}

	_, err = u.client.UpdateRelease(u.release, chartPath, helm.UpdateValueOverrides(rawVals), helm.UpgradeDryRun(u.dryRun), helm.UpgradeDisableHooks(u.disableHooks))
	if err != nil {
		return fmt.Errorf("UPGRADE FAILED: %v", prettyError(err))
	}

	success := u.release + " has been upgraded. Happy Helming!\n"
	fmt.Fprintf(u.out, success)

	// Print the status like status command does
	status, err := u.client.ReleaseStatus(u.release)
	if err != nil {
		return prettyError(err)
	}
	PrintStatus(u.out, status)

	return nil

}
