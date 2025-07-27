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

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	release "helm.sh/helm/v4/pkg/release/v1"
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
(scalars/objects/arrays) from the command line. Additionally, you can use '--set-json' and passing json object as a string.

    $ helm install -f myvalues.yaml myredis ./redis

or

    $ helm install --set name=prod myredis ./redis

or

    $ helm install --set-string long_int=1234567890 myredis ./redis

or

    $ helm install --set-file my_script=dothings.sh myredis ./redis

or

    $ helm install --set-json 'master.sidecars=[{"name":"sidecar","image":"myImage","imagePullPolicy":"Always","ports":[{"name":"portname","containerPort":1234}]}]' myredis ./redis

or

    $ helm install --set-json '{"master":{"sidecars":[{"name":"sidecar","image":"myImage","imagePullPolicy":"Always","ports":[{"name":"portname","containerPort":1234}]}]}}' myredis ./redis

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
		RunE: func(cmd *cobra.Command, args []string) error {
			registryClient, err := newRegistryClient(client.CertFile, client.KeyFile, client.CaFile,
				client.InsecureSkipTLSverify, client.PlainHTTP, client.Username, client.Password)
			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			client.SetRegistryClient(registryClient)

			dryRunStrategy, err := cmdGetDryRunFlagStrategy(cmd, false)
			if err != nil {
				return err
			}
			client.DryRunStrategy = dryRunStrategy

			rel, err := runInstall(args, client, valueOpts, out)
			if err != nil {
				return fmt.Errorf("INSTALLATION FAILED: %w", err)
			}

			return outfmt.Write(out, &statusPrinter{
				release:      rel,
				debug:        settings.Debug,
				showMetadata: false,
				hideNotes:    client.HideNotes,
				noColor:      settings.ShouldDisableColor(),
			})
		},
	}

	f := cmd.Flags()
	addInstallFlags(cmd, f, client, valueOpts, false)
	// hide-secret is not available in all places the install flags are used so
	// it is added separately
	f.BoolVar(&client.HideSecret, "hide-secret", false, "hide Kubernetes Secrets when also using the --dry-run flag")
	addDryRunFlag(cmd)
	bindOutputFlag(cmd, &outfmt)
	bindPostRenderFlag(cmd, &client.PostRenderer, settings)

	return cmd
}

func addInstallFlags(cmd *cobra.Command, f *pflag.FlagSet, client *action.Install, valueOpts *values.Options,
	isTemplateCommand bool) {
	f.BoolVar(&client.CreateNamespace, "create-namespace", false, "create the release namespace if not present")
	f.BoolVar(&client.ForceReplace, "force-replace", false, "force resource updates by replacement")
	f.BoolVar(&client.ForceReplace, "force", false, "deprecated")
	f.MarkDeprecated("force", "use --force-replace instead")
	f.BoolVar(&client.ForceConflicts, "force-conflicts", false, "if set server-side apply will force changes against conflicts")
	f.BoolVar(&client.ServerSideApply, "server-side", true, "object updates run in the server instead of the client")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "prevent hooks from running during install")
	f.BoolVar(&client.Replace, "replace", false, "reuse the given name, only if that name is a deleted release which remains in the history. This is unsafe in production")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.WaitForJobs, "wait-for-jobs", false, "if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVarP(&client.GenerateName, "generate-name", "g", false, "generate the name (and omit the NAME parameter)")
	f.StringVar(&client.NameTemplate, "name-template", "", "specify template used to name the release")
	f.StringVar(&client.Description, "description", "", "add a custom description")
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "update dependencies if they are missing before installing the chart")
	f.BoolVar(&client.DisableOpenAPIValidation, "disable-openapi-validation", false, "if set, the installation process will not validate rendered templates against the Kubernetes OpenAPI Schema")
	f.BoolVar(&client.RollbackOnFailure, "rollback-on-failure", false, "if set, Helm will rollback (uninstall) the installation upon failure. The --wait flag will be default to \"watcher\" if --rollback-on-failure is set")
	f.MarkDeprecated("atomic", "use --rollback-on-failure instead")
	f.BoolVar(&client.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed. By default, CRDs are installed if not already present")
	f.BoolVar(&client.SkipSchemaValidation, "skip-schema-validation", false, "if set, disables JSON schema validation")
	f.StringToStringVarP(&client.Labels, "labels", "l", nil, "Labels that would be added to release metadata. Should be divided by comma.")
	f.BoolVar(&client.EnableDNS, "enable-dns", false, "enable DNS lookups when rendering templates")
	f.BoolVar(&client.HideNotes, "hide-notes", false, "if set, do not show notes in install output. Does not affect presence in chart metadata")
	f.BoolVar(&client.TakeOwnership, "take-ownership", false, "if set, install will ignore the check for helm annotations and take ownership of the existing resources")

	// Set the --render-subchart-notes flag description based on the command. The template command requires some
	// additional information than the install/upgrade commands.
	renderSubchartNotesFlagDesc := "if set, render subchart notes along with the parent"
	if isTemplateCommand {
		renderSubchartNotesFlagDesc = fmt.Sprintf("%s. Note: This will only work if --notes flag is enabled.",
			renderSubchartNotesFlagDesc)
	}
	f.BoolVar(&client.SubNotes, "render-subchart-notes", false, renderSubchartNotesFlagDesc)

	addValueOptionsFlags(f, valueOpts)
	addChartPathOptionsFlags(f, &client.ChartPathOptions)
	AddWaitFlag(cmd, &client.WaitStrategy)
	cmd.MarkFlagsMutuallyExclusive("force-replace", "force-conflicts")
	cmd.MarkFlagsMutuallyExclusive("force", "force-conflicts")

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
	slog.Debug("Original chart version", "version", client.Version)
	if client.Version == "" && client.Devel {
		slog.Debug("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	name, chartRef, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name

	cp, err := client.LocateChart(chartRef, settings)
	if err != nil {
		return nil, err
	}

	slog.Debug("Chart path", "path", cp)

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

	ac, err := chart.NewAccessor(chartRequested)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(ac); err != nil {
		return nil, err
	}

	if ac.Deprecated() {
		slog.Warn("this chart is deprecated")
	}

	if req := ac.MetaDependencies(); req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              out,
					ChartPath:        cp,
					Keyring:          client.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					ContentCache:     settings.ContentCache,
					Debug:            settings.Debug,
					RegistryClient:   client.GetRegistryClient(),
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, fmt.Errorf("failed reloading chart after repo update: %w", err)
				}
			} else {
				return nil, fmt.Errorf("an error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies: %w", err)
			}
		}
	}

	client.Namespace = settings.Namespace()

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

	ri, err := client.RunWithContext(ctx, chartRequested, vals)
	rel, rerr := releaserToV1Release(ri)
	if rerr != nil {
		return nil, rerr
	}
	return rel, err
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch chart.Accessor) error {
	meta := ch.MetadataAsMap()

	switch meta["Type"] {
	case "", "application":
		return nil
	}
	return fmt.Errorf("%s charts are not installable", meta["Type"])
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
