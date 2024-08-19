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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

const installDesc = `
This command installs a chart archive.

The install argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'. You can use '--set-file' to set individual
values from a file when the value itself is too long for the command line
or is dynamically generated. You can also use '--set-json' to set json values
(scalars/objects/arrays) from the command line.

    $ helm install -f myvalues.yaml myredis ./redis

or

    $ helm install --set name=prod myredis ./redis

or

    $ helm install --set-string long_int=1234567890 myredis ./redis

or

    $ helm install --set-file my_script=dothings.sh myredis ./redis

or

    $ helm install --set-json 'master.sidecars=[{"name":"sidecar","image":"myImage","imagePullPolicy":"Always","ports":[{"name":"portname","containerPort":1234}]}]' myredis ./redis


You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

    $ helm install -f myvalues.yaml -f override.yaml  myredis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

    $ helm install --set foo=bar --set foo=newbar  myredis ./redis

Similarly, in the following example 'foo' is set to '["four"]': 

    $ helm install --set-json='foo=["one", "two", "three"]' --set-json='foo=["four"]' myredis ./redis

And in the following example, 'foo' is set to '{"key1":"value1","key2":"bar"}':

    $ helm install --set-json='foo={"key1":"value1","key2":"value2"}' --set-json='foo.key2="bar"' myredis ./redis

To check the generated manifests of a release without installing the chart,
the --debug and --dry-run flags can be combined.

The --dry-run flag will output all generated chart manifests, including Secrets
which can contain sensitive values. To hide Kubernetes Secrets use the
--hide-secret flag. Please carefully consider how and when these flags are used.

If --verify is set, the chart MUST have a provenance file, and the provenance
file MUST pass all verification steps.

There are six different ways you can express the chart you want to install:

1. By chart reference: helm install mymaria example/mariadb
2. By path to a packaged chart: helm install mynginx ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install mynginx ./nginx
4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx
6. By OCI registries: helm install mynginx --version 1.2.3 oci://example.com/charts/nginx

CHART REFERENCES

A chart reference is a convenient way of referencing a chart in a chart repository.

When you use a chart reference with a repo prefix ('example/mariadb'), Helm will look in the local
configuration for a chart repository named 'example', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest stable version of that chart
until you specify '--devel' flag to also include development version (alpha, beta, and release candidate releases), or
supply a version number with the '--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`

func newInstallCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewInstall(cfg)
	valueOpts := &values.Options{}
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:   "install [NAME] [CHART]",
		Short: "install a chart",
		Long:  installDesc,
		Args:  require.MinimumNArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compInstall(args, toComplete, client)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			registryClient, err := newRegistryClient(client.CertFile, client.KeyFile, client.CaFile,
				client.InsecureSkipTLSverify, client.PlainHTTP)
			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			client.SetRegistryClient(registryClient)

			// This is for the case where "" is specifically passed in as a
			// value. When there is no value passed in NoOptDefVal will be used
			// and it is set to client. See addInstallFlags.
			if client.DryRunOption == "" {
				client.DryRunOption = "none"
			}
			rel, err := runInstall(args, client, valueOpts, out)
			if err != nil {
				return errors.Wrap(err, "INSTALLATION FAILED")
			}

			return outfmt.Write(out, &statusPrinter{rel, settings.Debug, false, false, false})
		},
	}

	addInstallFlags(cmd, cmd.Flags(), client, valueOpts)
	// hide-secret is not available in all places the install flags are used so
	// it is added separately
	f := cmd.Flags()
	f.BoolVar(&client.HideSecret, "hide-secret", false, "hide Kubernetes Secrets when also using the --dry-run flag")
	bindOutputFlag(cmd, &outfmt)
	bindPostRenderFlag(cmd, &client.PostRenderer)

	return cmd
}

func addInstallFlags(cmd *cobra.Command, f *pflag.FlagSet, client *action.Install, valueOpts *values.Options) {
	f.BoolVar(&client.CreateNamespace, "create-namespace", false, "create the release namespace if not present")
	// --dry-run options with expected outcome:
	// - Not set means no dry run and server is contacted.
	// - Set with no value, a value of client, or a value of true and the server is not contacted
	// - Set with a value of false, none, or false and the server is contacted
	// The true/false part is meant to reflect some legacy behavior while none is equal to "".
	f.StringVar(&client.DryRunOption, "dry-run", "", "simulate an install. If --dry-run is set with no option being specified or as '--dry-run=client', it will not attempt cluster connections. Setting '--dry-run=server' allows attempting cluster connections.")
	f.Lookup("dry-run").NoOptDefVal = "client"
	f.BoolVar(&client.Force, "force", false, "force resource updates through a replacement strategy")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&client.Replace, "replace", false, "re-use the given name, only if that name is a deleted release which remains in the history. This is unsafe in production")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.Wait, "wait", false, "if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&client.WaitForJobs, "wait-for-jobs", false, "if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVarP(&client.GenerateName, "generate-name", "g", false, "generate the name (and omit the NAME parameter)")
	f.StringVar(&client.NameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&client.Description, "description", "", "add a custom description")
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "update dependencies if they are missing before installing the chart")
	f.BoolVar(&client.DisableOpenAPIValidation, "disable-openapi-validation", false, "if set, the installation process will not validate rendered templates against the Kubernetes OpenAPI Schema")
	f.BoolVar(&client.Atomic, "atomic", false, "if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used")
	f.BoolVar(&client.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed. By default, CRDs are installed if not already present")
	f.BoolVar(&client.SubNotes, "render-subchart-notes", false, "if set, render subchart notes along with the parent")
	f.StringToStringVarP(&client.Labels, "labels", "l", nil, "Labels that would be added to release metadata. Should be divided by comma.")
	f.BoolVar(&client.EnableDNS, "enable-dns", false, "enable DNS lookups when rendering templates")
	addValueOptionsFlags(f, valueOpts)
	addChartPathOptionsFlags(f, &client.ChartPathOptions)

	err := cmd.RegisterFlagCompletionFunc("version", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		requiredArgs := 2
		if client.GenerateName {
			requiredArgs = 1
		}
		if len(args) != requiredArgs {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return compVersionFlag(args[requiredArgs-1], toComplete)
	})

	if err != nil {
		log.Fatal(err)
	}
}

func runInstall(args []string, client *action.Install, valueOpts *values.Options, out io.Writer) (*release.Release, error) {
	debug("Original chart version: %q", client.Version)
	if client.Version == "" && client.Devel {
		debug("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	name, chart, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name

	cp, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return nil, err
	}

	debug("CHART PATH: %s\n", cp)

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	if chartRequested.Metadata.Deprecated {
		warning("This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			err = errors.Wrap(err, "An error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies")
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              out,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
					RegistryClient:   client.GetRegistryClient(),
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, err
			}
		}
	}

	client.Namespace = settings.Namespace()

	// Validate DryRunOption member is one of the allowed values
	if err := validateDryRunOptionFlag(client.DryRunOption); err != nil {
		return nil, err
	}

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Fprintf(out, "Release %s has been cancelled.\n", args[0])
		cancel()
	}()

	return client.RunWithContext(ctx, chartRequested, vals)
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

// Provide dynamic auto-completion for the install and template commands
func compInstall(args []string, toComplete string, client *action.Install) ([]string, cobra.ShellCompDirective) {
	requiredArgs := 1
	if client.GenerateName {
		requiredArgs = 0
	}
	if len(args) == requiredArgs {
		return compListCharts(toComplete, true)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func validateDryRunOptionFlag(dryRunOptionFlagValue string) error {
	// Validate dry-run flag value with a set of allowed value
	allowedDryRunValues := []string{"false", "true", "none", "client", "server"}
	isAllowed := false
	for _, v := range allowedDryRunValues {
		if dryRunOptionFlagValue == v {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return errors.New("Invalid dry-run flag. Flag must one of the following: false, true, none, client, server")
	}
	return nil
}
