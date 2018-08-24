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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/storage/driver"
)

const upgradeDesc = `
This command upgrades a release to a new version of a chart.

The upgrade arguments must be a release and chart. The chart
argument can be either: a chart reference('stable/mariadb'), a path to a chart directory,
a packaged chart, or a fully qualified URL. For chart references, the latest
version will be specified unless the '--version' flag is set.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force string
values, use '--set-string'.

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

	$ helm upgrade -f myvalues.yaml -f override.yaml redis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

	$ helm upgrade --set foo=bar --set foo=newbar redis ./redis
`

type upgradeOptions struct {
	devel        bool  // --devel
	disableHooks bool  // --disable-hooks
	dryRun       bool  // --dry-run
	force        bool  // --force
	install      bool  // --install
	recreate     bool  // --recreate-pods
	resetValues  bool  // --reset-values
	reuseValues  bool  // --reuse-values
	timeout      int64 // --timeout
	wait         bool  // --wait

	valuesOptions
	chartPathOptions

	release string
	chart   string

	client helm.Interface
}

func newUpgradeCmd(client helm.Interface, out io.Writer) *cobra.Command {
	o := &upgradeOptions{client: client}

	cmd := &cobra.Command{
		Use:   "upgrade [RELEASE] [CHART]",
		Short: "upgrade a release",
		Long:  upgradeDesc,
		Args:  require.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.version == "" && o.devel {
				debug("setting version to >0.0.0-0")
				o.version = ">0.0.0-0"
			}

			o.release = args[0]
			o.chart = args[1]
			o.client = ensureHelmClient(o.client, false)

			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.dryRun, "dry-run", false, "simulate an upgrade")
	f.BoolVar(&o.recreate, "recreate-pods", false, "performs pods restart for the resource if applicable")
	f.BoolVar(&o.force, "force", false, "force resource update through delete/recreate if needed")
	f.BoolVar(&o.disableHooks, "disable-hooks", false, "disable pre/post upgrade hooks. DEPRECATED. Use no-hooks")
	f.BoolVar(&o.disableHooks, "no-hooks", false, "disable pre/post upgrade hooks")
	f.BoolVarP(&o.install, "install", "i", false, "if a release by this name doesn't already exist, run an install")
	f.Int64Var(&o.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&o.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&o.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored.")
	f.BoolVar(&o.wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&o.devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")
	o.valuesOptions.addFlags(f)
	o.chartPathOptions.addFlags(f)

	return cmd
}

func (o *upgradeOptions) run(out io.Writer) error {
	chartPath, err := o.locateChart(o.chart)
	if err != nil {
		return err
	}

	if o.install {
		// If a release does not exist, install it. If another error occurs during
		// the check, ignore the error and continue with the upgrade.
		_, err := o.client.ReleaseHistory(o.release, 1)

		if err != nil && strings.Contains(err.Error(), driver.ErrReleaseNotFound(o.release).Error()) {
			fmt.Fprintf(out, "Release %q does not exist. Installing it now.\n", o.release)
			io := &installOptions{
				chartPath:        chartPath,
				client:           o.client,
				name:             o.release,
				dryRun:           o.dryRun,
				disableHooks:     o.disableHooks,
				timeout:          o.timeout,
				wait:             o.wait,
				valuesOptions:    o.valuesOptions,
				chartPathOptions: o.chartPathOptions,
			}
			return io.run(out)
		}
	}

	rawVals, err := o.mergedValues()
	if err != nil {
		return err
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	ch, err := loader.Load(chartPath)
	if err != nil {
		return err
	}
	if req := ch.Requirements; req != nil {
		if err := checkDependencies(ch, req); err != nil {
			return err
		}
	}

	resp, err := o.client.UpdateRelease(
		o.release,
		chartPath,
		helm.UpdateValueOverrides(rawVals),
		helm.UpgradeDryRun(o.dryRun),
		helm.UpgradeRecreate(o.recreate),
		helm.UpgradeForce(o.force),
		helm.UpgradeDisableHooks(o.disableHooks),
		helm.UpgradeTimeout(o.timeout),
		helm.ResetValues(o.resetValues),
		helm.ReuseValues(o.reuseValues),
		helm.UpgradeWait(o.wait))
	if err != nil {
		return errors.Wrap(err, "UPGRADE FAILED")
	}

	if settings.Debug {
		printRelease(out, resp)
	}

	fmt.Fprintf(out, "Release %q has been upgraded. Happy Helming!\n", o.release)

	// Print the status like status command does
	status, err := o.client.ReleaseStatus(o.release, 0)
	if err != nil {
		return err
	}
	PrintStatus(out, status)

	return nil
}
