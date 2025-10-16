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

	"helm.sh/helm/v4/pkg/action"
	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	ri "helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	"helm.sh/helm/v4/pkg/storage/driver"
)

const upgradeDesc = `
This command upgrades a release to a new version of a chart.

The upgrade arguments must be a release and chart. The chart
argument can be either: a chart reference('example/mariadb'), a path to a chart directory,
a packaged chart, or a fully qualified URL. For chart references, the latest
version will be specified unless the '--version' flag is set.

To override values in a chart, use the '-f'/'--values' flag to provide a file or
the '--set' flag to pass configuration directly from the command line. To force
a string value, use '--set-string'. Use '--set-file' when a value is too long
for the command line or dynamically generated. You can also use '--set-json' to
provide JSON values (scalars, objects, or arrays) from the command line, either
as a direct argument or by passing a JSON object as a string. Alternatively, use
the '-d'/'--values-directory' flag to specify a directory containing YAML files
when you have many values or shared configurations split across files.

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

    $ helm upgrade -f myvalues.yaml -f override.yaml redis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

    $ helm upgrade --set foo=bar --set foo=newbar redis ./redis

You can update the values for an existing release with this command as well via the
'--reuse-values' flag. The 'RELEASE' and 'CHART' arguments should be set to the original
parameters, and existing values will be merged with any values set via '--values'/'-f'
or '--set' flags. Priority is given to new values.

    $ helm upgrade --reuse-values --set foo=bar --set foo=newbar redis ./redis

Key details about flag '-d'/'--values-directory':
- **Purpose:**
  - Specify a directory containing values YAML files.
- **Behavior:**
  - All YAML files in the directory and its nested subdirectories are loaded
    recursively. Non-YAML files are skipped.
  - Files within a single directory are processed in **lexicographical order**,
    with later files overriding earlier ones when keys overlap.
- **Precedence:**
  - This flag has the **lower precedence** than other value flags
    ('-f'/'--values', '--set', '--set-string', '--set-file', '--set-json',
    '--set-literal'). Values from these other flags override values from files
    in the specified directory.
  - Exception: The chart's default 'values.yaml' has a **lower precedence** than
    the '-d'/'--values-directory' flag, i.e., the values from files in the
    directory can override it. To let default values override directory files,
    include 'values.yaml' explicitly via '-f'/'--values'.
- **Multiple Directories:**
  - The flag can be specified **multiple times**.
  - When multiple directories are provided, files in directories specified later
    override values from earlier directories.
  - **Lexicographical ordering** applies within each directory (and its nested
    subdirectories) and not between directories specified with multiple
    '-d'/'--values-directory' flags.

    $ helm upgrade -d default-values/ -d prod-overrides/ myredis ./redis

The --dry-run flag will output all generated chart manifests, including Secrets
which can contain sensitive values. To hide Kubernetes Secrets use the
--hide-secret flag. Please carefully consider how and when these flags are used.
`

func newUpgradeCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewUpgrade(cfg)
	valueOpts := &values.Options{}
	var outfmt output.Format
	var createNamespace bool

	cmd := &cobra.Command{
		Use:   "upgrade [RELEASE] [CHART]",
		Short: "upgrade a release",
		Long:  upgradeDesc,
		Args:  require.ExactArgs(2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListReleases(toComplete, args, cfg)
			}
			if len(args) == 1 {
				return compListCharts(toComplete, true)
			}
			return noMoreArgsComp()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Namespace = settings.Namespace()

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

			// Fixes #7002 - Support reading values from STDIN for `upgrade` command
			// Must load values AFTER determining if we have to call install so that values loaded from stdin are not read twice
			if client.Install {
				// If a release does not exist, install it.
				histClient := action.NewHistory(cfg)
				histClient.Max = 1
				versions, err := histClient.Run(args[0])
				if err == driver.ErrReleaseNotFound || isReleaseUninstalled(versions) {
					// Only print this to stdout for table output
					if outfmt == output.Table {
						fmt.Fprintf(out, "Release %q does not exist. Installing it now.\n", args[0])
					}
					instClient := action.NewInstall(cfg)
					instClient.CreateNamespace = createNamespace
					instClient.ChartPathOptions = client.ChartPathOptions
					instClient.ForceReplace = client.ForceReplace
					instClient.DryRunStrategy = client.DryRunStrategy
					instClient.DisableHooks = client.DisableHooks
					instClient.SkipCRDs = client.SkipCRDs
					instClient.Timeout = client.Timeout
					instClient.WaitStrategy = client.WaitStrategy
					instClient.WaitForJobs = client.WaitForJobs
					instClient.Devel = client.Devel
					instClient.Namespace = client.Namespace
					instClient.RollbackOnFailure = client.RollbackOnFailure
					instClient.PostRenderer = client.PostRenderer
					instClient.DisableOpenAPIValidation = client.DisableOpenAPIValidation
					instClient.SubNotes = client.SubNotes
					instClient.HideNotes = client.HideNotes
					instClient.SkipSchemaValidation = client.SkipSchemaValidation
					instClient.Description = client.Description
					instClient.DependencyUpdate = client.DependencyUpdate
					instClient.Labels = client.Labels
					instClient.EnableDNS = client.EnableDNS
					instClient.HideSecret = client.HideSecret
					instClient.TakeOwnership = client.TakeOwnership

					if isReleaseUninstalled(versions) {
						instClient.Replace = true
					}

					rel, err := runInstall(args, instClient, valueOpts, out)
					if err != nil {
						return err
					}
					return outfmt.Write(out, &statusPrinter{
						release:      rel,
						debug:        settings.Debug,
						showMetadata: false,
						hideNotes:    instClient.HideNotes,
						noColor:      settings.ShouldDisableColor(),
					})
				} else if err != nil {
					return err
				}
			}

			if client.Version == "" && client.Devel {
				slog.Debug("setting version to >0.0.0-0")
				client.Version = ">0.0.0-0"
			}

			chartPath, err := client.LocateChart(args[1], settings)
			if err != nil {
				return err
			}

			p := getter.All(settings)
			vals, err := valueOpts.MergeValues(p)
			if err != nil {
				return err
			}

			// Check chart dependencies to make sure all are present in /charts
			ch, err := loader.Load(chartPath)
			if err != nil {
				return err
			}

			ac, err := ci.NewAccessor(ch)
			if err != nil {
				return err
			}
			if req := ac.MetaDependencies(); req != nil {
				if err := action.CheckDependencies(ch, req); err != nil {
					err = fmt.Errorf("an error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies: %w", err)
					if client.DependencyUpdate {
						man := &downloader.Manager{
							Out:              out,
							ChartPath:        chartPath,
							Keyring:          client.Keyring,
							SkipUpdate:       false,
							Getters:          p,
							RepositoryConfig: settings.RepositoryConfig,
							RepositoryCache:  settings.RepositoryCache,
							ContentCache:     settings.ContentCache,
							Debug:            settings.Debug,
						}
						if err := man.Update(); err != nil {
							return err
						}
						// Reload the chart with the updated Chart.lock file.
						if ch, err = loader.Load(chartPath); err != nil {
							return fmt.Errorf("failed reloading chart after repo update: %w", err)
						}
					} else {
						return err
					}
				}
			}

			if ac.Deprecated() {
				slog.Warn("this chart is deprecated")
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

			rel, err := client.RunWithContext(ctx, args[0], ch, vals)
			if err != nil {
				return fmt.Errorf("UPGRADE FAILED: %w", err)
			}

			if outfmt == output.Table {
				fmt.Fprintf(out, "Release %q has been upgraded. Happy Helming!\n", args[0])
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
	f.BoolVar(&createNamespace, "create-namespace", false, "if --install is set, create the release namespace if not present")
	f.BoolVarP(&client.Install, "install", "i", false, "if a release by this name doesn't already exist, run an install")
	f.BoolVar(&client.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.BoolVar(&client.HideSecret, "hide-secret", false, "hide Kubernetes Secrets when also using the --dry-run flag")
	f.BoolVar(&client.ForceReplace, "force-replace", false, "force resource updates by replacement")
	f.BoolVar(&client.ForceReplace, "force", false, "deprecated")
	f.MarkDeprecated("force", "use --force-replace instead")
	f.BoolVar(&client.ForceConflicts, "force-conflicts", false, "if set server-side apply will force changes against conflicts")
	f.StringVar(&client.ServerSideApply, "server-side", "auto", "must be \"true\", \"false\" or \"auto\". Object updates run in the server instead of the client (\"auto\" defaults the value from the previous chart release's method)")
	f.BoolVar(&client.DisableHooks, "no-hooks", false, "disable pre/post upgrade hooks")
	f.BoolVar(&client.DisableOpenAPIValidation, "disable-openapi-validation", false, "if set, the upgrade process will not validate rendered templates against the Kubernetes OpenAPI Schema")
	f.BoolVar(&client.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed when an upgrade is performed with install flag enabled. By default, CRDs are installed if not already present, when an upgrade is performed with install flag enabled")
	f.DurationVar(&client.Timeout, "timeout", 300*time.Second, "time to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.ResetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&client.ReuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored")
	f.BoolVar(&client.ResetThenReuseValues, "reset-then-reuse-values", false, "when upgrading, reset the values to the ones built into the chart, apply the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' or '--reuse-values' is specified, this is ignored")
	f.BoolVar(&client.WaitForJobs, "wait-for-jobs", false, "if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout")
	f.BoolVar(&client.RollbackOnFailure, "rollback-on-failure", false, "if set, Helm will rollback the upgrade to previous success release upon failure. The --wait flag will be defaulted to \"watcher\" if --rollback-on-failure is set")
	f.BoolVar(&client.RollbackOnFailure, "atomic", false, "deprecated")
	f.MarkDeprecated("atomic", "use --rollback-on-failure instead")
	f.IntVar(&client.MaxHistory, "history-max", settings.MaxHistory, "limit the maximum number of revisions saved per release. Use 0 for no limit")
	f.BoolVar(&client.CleanupOnFail, "cleanup-on-fail", false, "allow deletion of new resources created in this upgrade when upgrade fails")
	f.BoolVar(&client.SubNotes, "render-subchart-notes", false, "if set, render subchart notes along with the parent")
	f.BoolVar(&client.HideNotes, "hide-notes", false, "if set, do not show notes in upgrade output. Does not affect presence in chart metadata")
	f.BoolVar(&client.SkipSchemaValidation, "skip-schema-validation", false, "if set, disables JSON schema validation")
	f.StringToStringVarP(&client.Labels, "labels", "l", nil, "Labels that would be added to release metadata. Should be separated by comma. Original release labels will be merged with upgrade labels. You can unset label using null.")
	f.StringVar(&client.Description, "description", "", "add a custom description")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "update dependencies if they are missing before installing the chart")
	f.BoolVar(&client.EnableDNS, "enable-dns", false, "enable DNS lookups when rendering templates")
	f.BoolVar(&client.TakeOwnership, "take-ownership", false, "if set, upgrade will ignore the check for helm annotations and take ownership of the existing resources")
	addDryRunFlag(cmd)
	addChartPathOptionsFlags(f, &client.ChartPathOptions)
	addValueOptionsFlags(f, valueOpts)
	bindOutputFlag(cmd, &outfmt)
	bindPostRenderFlag(cmd, &client.PostRenderer, settings)
	AddWaitFlag(cmd, &client.WaitStrategy)
	cmd.MarkFlagsMutuallyExclusive("force-replace", "force-conflicts")
	cmd.MarkFlagsMutuallyExclusive("force", "force-conflicts")

	err := cmd.RegisterFlagCompletionFunc("version", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return compVersionFlag(args[1], toComplete)
	})
	if err != nil {
		log.Fatal(err)
	}

	return cmd
}

func isReleaseUninstalled(versionsi []ri.Releaser) bool {
	versions, err := releaseListToV1List(versionsi)
	if err != nil {
		slog.Error("cannot convert release list to v1 release list", "error", err)
		return false
	}
	return len(versions) > 0 && versions[len(versions)-1].Info.Status == common.StatusUninstalled
}
