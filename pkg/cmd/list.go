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
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	coloroutput "helm.sh/helm/v4/internal/cli/output"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
	release "helm.sh/helm/v4/pkg/release/v1"
)

var listHelp = `
This command lists all of the releases for a specified namespace (uses current namespace context if namespace not specified).

By default, it lists all releases in any status including deployed, failed, 
pending-upgrade, pending-install, pending-rollback, uninstalled, uninstalling, 
superseded, and unknown. Individual status filters like '--deployed', '--failed',
'--pending', '--uninstalled', '--superseded', and '--uninstalling' can be used 
to show only releases in specific states. Such flags can be combined:
'--deployed --failed'.

By default, items are sorted alphabetically. Use the '-d' flag to sort by
release date.

If the --filter flag is provided, it will be treated as a filter. Filters are
regular expressions (Perl compatible) that are applied to the list of releases.
Only items that match the filter will be returned.

    $ helm list --filter 'ara[a-z]+'
    NAME                UPDATED                                  CHART
    maudlin-arachnid    2020-06-18 14:17:46.125134977 +0000 UTC  alpine-0.1.0

If no results are found, 'helm list' will exit 0, but with no output (or in
the case of no '-q' flag, only headers).

By default, up to 256 items may be returned. To limit this, use the '--max' flag.
Setting '--max' to 0 will not return all results. Rather, it will return the
server's default, which may be much higher than 256. Pairing the '--max'
flag with the '--offset' flag allows you to page through results.
`

func newListCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewList(cfg)
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:               "list",
		Short:             "list releases",
		Long:              listHelp,
		Aliases:           []string{"ls"},
		Args:              require.NoArgs,
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if client.AllNamespaces {
				if err := cfg.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER")); err != nil {
					return err
				}
			}
			client.SetStateMask()

			results, err := client.Run()
			if err != nil {
				return err
			}

			if client.Short {
				names := make([]string, 0, len(results))
				for _, res := range results {
					names = append(names, res.Name)
				}

				outputFlag := cmd.Flag("output")

				switch outputFlag.Value.String() {
				case "json":
					output.EncodeJSON(out, names)
					return nil
				case "yaml":
					output.EncodeYAML(out, names)
					return nil
				case "table":
					for _, res := range results {
						fmt.Fprintln(out, res.Name)
					}
					return nil
				}
			}

			return outfmt.Write(out, newReleaseListWriter(results, client.TimeFormat, client.NoHeaders, settings.ShouldDisableColor()))
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&client.Short, "short", "q", false, "output short (quiet) listing format")
	f.BoolVarP(&client.NoHeaders, "no-headers", "", false, "don't print headers when using the default output format")
	f.StringVar(&client.TimeFormat, "time-format", "", `format time using golang time formatter. Example: --time-format "2006-01-02 15:04:05Z0700"`)
	f.BoolVarP(&client.ByDate, "date", "d", false, "sort by release date")
	f.BoolVarP(&client.SortReverse, "reverse", "r", false, "reverse the sort order")
	f.BoolVar(&client.Uninstalled, "uninstalled", false, "show only uninstalled releases (if 'helm uninstall --keep-history' was used)")
	f.BoolVar(&client.Superseded, "superseded", false, "show only superseded releases")
	f.BoolVar(&client.Uninstalling, "uninstalling", false, "show only releases that are currently being uninstalled")
	f.BoolVar(&client.Deployed, "deployed", false, "show only deployed releases")
	f.BoolVar(&client.Failed, "failed", false, "show only failed releases")
	f.BoolVar(&client.Pending, "pending", false, "show only pending releases")
	f.BoolVarP(&client.AllNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	f.IntVarP(&client.Limit, "max", "m", 256, "maximum number of releases to fetch")
	f.IntVar(&client.Offset, "offset", 0, "next release index in the list, used to offset from start value")
	f.StringVarP(&client.Filter, "filter", "f", "", "a regular expression (Perl compatible). Any releases that match the expression will be included in the results")
	f.StringVarP(&client.Selector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Works only for secret(default) and configmap storage backends.")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

type releaseElement struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"app_version"`
}

type releaseListWriter struct {
	releases  []releaseElement
	noHeaders bool
	noColor   bool
}

func newReleaseListWriter(releases []*release.Release, timeFormat string, noHeaders bool, noColor bool) *releaseListWriter {
	// Initialize the array so no results returns an empty array instead of null
	elements := make([]releaseElement, 0, len(releases))
	for _, r := range releases {
		element := releaseElement{
			Name:       r.Name,
			Namespace:  r.Namespace,
			Revision:   strconv.Itoa(r.Version),
			Status:     r.Info.Status.String(),
			Chart:      formatChartName(r.Chart),
			AppVersion: formatAppVersion(r.Chart),
		}

		t := "-"
		if tspb := r.Info.LastDeployed; !tspb.IsZero() {
			if timeFormat != "" {
				t = tspb.Format(timeFormat)
			} else {
				t = tspb.String()
			}
		}
		element.Updated = t

		elements = append(elements, element)
	}
	return &releaseListWriter{elements, noHeaders, noColor}
}

func (w *releaseListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	if !w.noHeaders {
		table.AddRow(
			coloroutput.ColorizeHeader("NAME", w.noColor),
			coloroutput.ColorizeHeader("NAMESPACE", w.noColor),
			coloroutput.ColorizeHeader("REVISION", w.noColor),
			coloroutput.ColorizeHeader("UPDATED", w.noColor),
			coloroutput.ColorizeHeader("STATUS", w.noColor),
			coloroutput.ColorizeHeader("CHART", w.noColor),
			coloroutput.ColorizeHeader("APP VERSION", w.noColor),
		)
	}
	for _, r := range w.releases {
		// Parse the status string back to a release.Status to use color
		var status release.Status
		switch r.Status {
		case "deployed":
			status = release.StatusDeployed
		case "failed":
			status = release.StatusFailed
		case "pending-install":
			status = release.StatusPendingInstall
		case "pending-upgrade":
			status = release.StatusPendingUpgrade
		case "pending-rollback":
			status = release.StatusPendingRollback
		case "uninstalling":
			status = release.StatusUninstalling
		case "uninstalled":
			status = release.StatusUninstalled
		case "superseded":
			status = release.StatusSuperseded
		case "unknown":
			status = release.StatusUnknown
		default:
			status = release.Status(r.Status)
		}
		table.AddRow(r.Name, coloroutput.ColorizeNamespace(r.Namespace, w.noColor), r.Revision, r.Updated, coloroutput.ColorizeStatus(status, w.noColor), r.Chart, r.AppVersion)
	}
	return output.EncodeTable(out, table)
}

func (w *releaseListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, w.releases)
}

func (w *releaseListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, w.releases)
}

// Returns all releases from 'releases', except those with names matching 'ignoredReleases'
func filterReleases(releases []*release.Release, ignoredReleaseNames []string) []*release.Release {
	// if ignoredReleaseNames is nil, just return releases
	if ignoredReleaseNames == nil {
		return releases
	}

	var filteredReleases []*release.Release
	for _, rel := range releases {
		found := slices.Contains(ignoredReleaseNames, rel.Name)
		if !found {
			filteredReleases = append(filteredReleases, rel)
		}
	}

	return filteredReleases
}

// Provide dynamic auto-completion for release names
func compListReleases(toComplete string, ignoredReleaseNames []string, cfg *action.Configuration) ([]string, cobra.ShellCompDirective) {
	cobra.CompDebugln(fmt.Sprintf("compListReleases with toComplete %s", toComplete), settings.Debug)

	client := action.NewList(cfg)
	client.All = true
	client.Limit = 0
	// Do not filter so as to get the entire list of releases.
	// This will allow zsh and fish to match completion choices
	// on other criteria then prefix.  For example:
	//   helm status ingress<TAB>
	// can match
	//   helm status nginx-ingress
	//
	// client.Filter = fmt.Sprintf("^%s", toComplete)

	client.SetStateMask()
	releases, err := client.Run()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}

	var choices []string
	filteredReleases := filterReleases(releases, ignoredReleaseNames)
	for _, rel := range filteredReleases {
		choices = append(choices,
			fmt.Sprintf("%s\t%s-%s -> %s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version, rel.Info.Status.String()))
	}

	return choices, cobra.ShellCompDirectiveNoFileComp
}
