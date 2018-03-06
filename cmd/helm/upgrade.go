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

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
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
or use the '--set' flag and pass configuration from the command line.

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

	$ helm upgrade -f myvalues.yaml -f override.yaml redis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

	$ helm upgrade --set foo=bar --set foo=newbar redis ./redis
`

type upgradeCmd struct {
	release      string
	chart        string
	out          io.Writer
	client       helm.Interface
	dryRun       bool
	recreate     bool
	force        bool
	disableHooks bool
	valueFiles   valueFiles
	values       []string
	verify       bool
	keyring      string
	install      bool
	namespace    string
	version      string
	timeout      int64
	resetValues  bool
	reuseValues  bool
	wait         bool
	repoURL      string
	devel        bool

	certFile string
	keyFile  string
	caFile   string
}

func newUpgradeCmd(client helm.Interface, out io.Writer) *cobra.Command {

	upgrade := &upgradeCmd{
		out:    out,
		client: client,
	}

	cmd := &cobra.Command{
		Use:     "upgrade [RELEASE] [CHART]",
		Short:   "upgrade a release",
		Long:    upgradeDesc,
		PreRunE: func(_ *cobra.Command, _ []string) error { return setupConnection() },
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name", "chart path"); err != nil {
				return err
			}

			if upgrade.version == "" && upgrade.devel {
				debug("setting version to >0.0.0-0")
				upgrade.version = ">0.0.0-0"
			}

			upgrade.release = args[0]
			upgrade.chart = args[1]
			upgrade.client = ensureHelmClient(upgrade.client)

			return upgrade.run()
		},
	}

	f := cmd.Flags()
	f.VarP(&upgrade.valueFiles, "values", "f", "specify values in a YAML file or a URL(can specify multiple)")
	f.BoolVar(&upgrade.dryRun, "dry-run", false, "simulate an upgrade")
	f.BoolVar(&upgrade.recreate, "recreate-pods", false, "performs pods restart for the resource if applicable")
	f.BoolVar(&upgrade.force, "force", false, "force resource update through delete/recreate if needed")
	f.StringArrayVar(&upgrade.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&upgrade.disableHooks, "disable-hooks", false, "disable pre/post upgrade hooks. DEPRECATED. Use no-hooks")
	f.BoolVar(&upgrade.disableHooks, "no-hooks", false, "disable pre/post upgrade hooks")
	f.BoolVar(&upgrade.verify, "verify", false, "verify the provenance of the chart before upgrading")
	f.StringVar(&upgrade.keyring, "keyring", defaultKeyring(), "path to the keyring that contains public signing keys")
	f.BoolVarP(&upgrade.install, "install", "i", false, "if a release by this name doesn't already exist, run an install")
	f.StringVar(&upgrade.namespace, "namespace", "", "namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace")
	f.StringVar(&upgrade.version, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")
	f.Int64Var(&upgrade.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&upgrade.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&upgrade.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values, and merge in any new values. If '--reset-values' is specified, this is ignored.")
	f.BoolVar(&upgrade.wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.StringVar(&upgrade.repoURL, "repo", "", "chart repository url where to locate the requested chart")
	f.StringVar(&upgrade.certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&upgrade.keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&upgrade.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&upgrade.devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")

	f.MarkDeprecated("disable-hooks", "use --no-hooks instead")

	return cmd
}

func (u *upgradeCmd) run() error {
	chartPath, err := locateChartPath(u.repoURL, u.chart, u.version, u.verify, u.keyring, u.certFile, u.keyFile, u.caFile)
	if err != nil {
		return err
	}

	if u.install {
		// If a release does not exist, install it. If another error occurs during
		// the check, ignore the error and continue with the upgrade.
		//
		// The returned error is a grpc.rpcError that wraps the message from the original error.
		// So we're stuck doing string matching against the wrapped error, which is nested somewhere
		// inside of the grpc.rpcError message.
		releaseHistory, err := u.client.ReleaseHistory(u.release, helm.WithMaxHistory(1))

		if err == nil {
			previousReleaseNamespace := releaseHistory.Releases[0].Namespace
			if previousReleaseNamespace != u.namespace {
				fmt.Fprintf(u.out, "WARNING: Namespace doesn't match with previous. Release will be deployed to %s\n", previousReleaseNamespace)
			}
		}

		if err != nil && strings.Contains(err.Error(), driver.ErrReleaseNotFound(u.release).Error()) {
			fmt.Fprintf(u.out, "Release %q does not exist. Installing it now.\n", u.release)
			ic := &installCmd{
				chartPath:    chartPath,
				client:       u.client,
				out:          u.out,
				name:         u.release,
				valueFiles:   u.valueFiles,
				dryRun:       u.dryRun,
				verify:       u.verify,
				disableHooks: u.disableHooks,
				keyring:      u.keyring,
				values:       u.values,
				namespace:    u.namespace,
				timeout:      u.timeout,
				wait:         u.wait,
			}
			return ic.run()
		}
	}

	rawVals, err := vals(u.valueFiles, u.values)
	if err != nil {
		return err
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	if ch, err := chartutil.Load(chartPath); err == nil {
		if req, err := chartutil.LoadRequirements(ch); err == nil {
			if err := checkDependencies(ch, req); err != nil {
				return err
			}
		} else if err != chartutil.ErrRequirementsNotFound {
			return fmt.Errorf("cannot load requirements: %v", err)
		}
	} else {
		return prettyError(err)
	}

	resp, err := u.client.UpdateRelease(
		u.release,
		chartPath,
		helm.UpdateValueOverrides(rawVals),
		helm.UpgradeDryRun(u.dryRun),
		helm.UpgradeRecreate(u.recreate),
		helm.UpgradeForce(u.force),
		helm.UpgradeDisableHooks(u.disableHooks),
		helm.UpgradeTimeout(u.timeout),
		helm.ResetValues(u.resetValues),
		helm.ReuseValues(u.reuseValues),
		helm.UpgradeWait(u.wait))
	if err != nil {
		return fmt.Errorf("UPGRADE FAILED: %v", prettyError(err))
	}

	if settings.Debug {
		printRelease(u.out, resp.Release)
	}

	fmt.Fprintf(u.out, "Release %q has been upgraded. Happy Helming!\n", u.release)

	// Print the status like status command does
	status, err := u.client.ReleaseStatus(u.release)
	if err != nil {
		return prettyError(err)
	}
	PrintStatus(u.out, status)

	return nil
}
