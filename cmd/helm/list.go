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
	"os"
	"strconv"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/release"
)

var listHelp = `
This command lists all of the releases for a specified namespace (uses current namespace context if namespace not specified).

By default, it lists only releases that are deployed or failed. Flags like
'--uninstalled' and '--all' will alter this behavior. Such flags can be combined:
'--uninstalled --failed'.

By default, items are sorted alphabetically. Use the '-d' flag to sort by
release date.

If the --filter flag is provided, it will be treated as a filter. Filters are
regular expressions (Perl compatible) that are applied to the list of releases.
Only items that match the filter will be returned.

    $ helm list --filter 'ara[a-z]+'
    NAME                UPDATED                     CHART
    maudlin-arachnid    Mon May  9 16:07:08 2016    alpine-0.1.0

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
		Use:     "list",
		Short:   "list releases",
		Long:    listHelp,
		Aliases: []string{"ls"},
		Args:    require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client.AllNamespaces {
				if err := cfg.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER"), debug); err != nil {
					return err
				}
			}
			client.SetStateMask()

			results, err := client.Run()
			if err != nil {
				return err
			}

			if client.Short {

				names := make([]string, 0)
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
				default:
					return outfmt.Write(out, newReleaseListWriter(results))
				}
			}

			return outfmt.Write(out, newReleaseListWriter(results))
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&client.Short, "short", "q", false, "output short (quiet) listing format")
	f.BoolVarP(&client.ByDate, "date", "d", false, "sort by release date")
	f.BoolVarP(&client.SortReverse, "reverse", "r", false, "reverse the sort order")
	f.BoolVarP(&client.All, "all", "a", false, "show all releases without any filter applied")
	f.BoolVar(&client.Uninstalled, "uninstalled", false, "show uninstalled releases (if 'helm uninstall --keep-history' was used)")
	f.BoolVar(&client.Superseded, "superseded", false, "show superseded releases")
	f.BoolVar(&client.Uninstalling, "uninstalling", false, "show releases that are currently being uninstalled")
	f.BoolVar(&client.Deployed, "deployed", false, "show deployed releases. If no other is specified, this will be automatically enabled")
	f.BoolVar(&client.Failed, "failed", false, "show failed releases")
	f.BoolVar(&client.Pending, "pending", false, "show pending releases")
	f.BoolVarP(&client.AllNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	f.IntVarP(&client.Limit, "max", "m", 256, "maximum number of releases to fetch")
	f.IntVar(&client.Offset, "offset", 0, "next release name in the list, used to offset from start value")
	f.StringVarP(&client.Filter, "filter", "f", "", "a regular expression (Perl compatible). Any releases that match the expression will be included in the results")
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
	releases []releaseElement
}

func newReleaseListWriter(releases []*release.Release) *releaseListWriter {
	// Initialize the array so no results returns an empty array instead of null
	elements := make([]releaseElement, 0, len(releases))
	for _, r := range releases {
		element := releaseElement{
			Name:       r.Name,
			Namespace:  r.Namespace,
			Revision:   strconv.Itoa(r.Version),
			Status:     r.Info.Status.String(),
			Chart:      fmt.Sprintf("%s-%s", r.Chart.Metadata.Name, r.Chart.Metadata.Version),
			AppVersion: r.Chart.Metadata.AppVersion,
		}
		t := "-"
		if tspb := r.Info.LastDeployed; !tspb.IsZero() {
			t = tspb.String()
		}
		element.Updated = t
		elements = append(elements, element)
	}
	return &releaseListWriter{elements}
}

func (r *releaseListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("NAME", "NAMESPACE", "REVISION", "UPDATED", "STATUS", "CHART", "APP VERSION")
	for _, r := range r.releases {
		table.AddRow(r.Name, r.Namespace, r.Revision, r.Updated, r.Status, r.Chart, r.AppVersion)
	}
	return output.EncodeTable(out, table)
}

func (r *releaseListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, r.releases)
}

func (r *releaseListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, r.releases)
}

// Provide dynamic auto-completion for release names
func compListReleases(toComplete string, cfg *action.Configuration) ([]string, completion.BashCompDirective) {
	completion.CompDebugln(fmt.Sprintf("compListReleases with toComplete %s", toComplete))

	client := action.NewList(cfg)
	client.All = true
	client.Limit = 0
	client.Filter = fmt.Sprintf("^%s", toComplete)

	client.SetStateMask()
	results, err := client.Run()
	if err != nil {
		return nil, completion.BashCompDirectiveDefault
	}

	var choices []string
	for _, res := range results {
		choices = append(choices, res.Name)
	}

	return choices, completion.BashCompDirectiveNoFileComp
}
