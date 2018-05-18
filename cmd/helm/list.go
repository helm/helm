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

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

var listHelp = `
This command lists all of the releases.

By default, it lists only releases that are deployed or failed. Flags like
'--deleted' and '--all' will alter this behavior. Such flags can be combined:
'--deleted --failed'.

By default, items are sorted alphabetically. Use the '-d' flag to sort by
release date.

If an argument is provided, it will be treated as a filter. Filters are
regular expressions (Perl compatible) that are applied to the list of releases.
Only items that match the filter will be returned.

	$ helm list 'ara[a-z]+'
	NAME            	UPDATED                 	CHART
	maudlin-arachnid	Mon May  9 16:07:08 2016	alpine-0.1.0

If no results are found, 'helm list' will exit 0, but with no output (or in
the case of no '-q' flag, only headers).

By default, up to 256 items may be returned. To limit this, use the '--max' flag.
Setting '--max' to 0 will not return all results. Rather, it will return the
server's default, which may be much higher than 256. Pairing the '--max'
flag with the '--offset' flag allows you to page through results.
`

type listOptions struct {
	// flags
	all           bool   // --all
	allNamespaces bool   // --all-namespaces
	byDate        bool   // --date
	colWidth      uint   // --col-width
	deleted       bool   // --deleted
	deleting      bool   // --deleting
	deployed      bool   // --deployed
	failed        bool   // --failed
	limit         int    // --max
	offset        string // --offset
	pending       bool   // --pending
	short         bool   // --short
	sortDesc      bool   // --reverse
	superseded    bool   // --superseded

	filter string

	client helm.Interface
}

func newListCmd(client helm.Interface, out io.Writer) *cobra.Command {
	o := &listOptions{client: client}

	cmd := &cobra.Command{
		Use:     "list [FILTER]",
		Short:   "list releases",
		Long:    listHelp,
		Aliases: []string{"ls"},
		Args:    require.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.filter = strings.Join(args, " ")
			}
			o.client = ensureHelmClient(o.client, o.allNamespaces)
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.short, "short", "q", false, "output short (quiet) listing format")
	f.BoolVarP(&o.byDate, "date", "d", false, "sort by release date")
	f.BoolVarP(&o.sortDesc, "reverse", "r", false, "reverse the sort order")
	f.IntVarP(&o.limit, "max", "m", 256, "maximum number of releases to fetch")
	f.StringVarP(&o.offset, "offset", "o", "", "next release name in the list, used to offset from start value")
	f.BoolVarP(&o.all, "all", "a", false, "show all releases, not just the ones marked deployed")
	f.BoolVar(&o.deleted, "deleted", false, "show deleted releases")
	f.BoolVar(&o.superseded, "superseded", false, "show superseded releases")
	f.BoolVar(&o.deleting, "deleting", false, "show releases that are currently being deleted")
	f.BoolVar(&o.deployed, "deployed", false, "show deployed releases. If no other is specified, this will be automatically enabled")
	f.BoolVar(&o.failed, "failed", false, "show failed releases")
	f.BoolVar(&o.pending, "pending", false, "show pending releases")
	f.UintVar(&o.colWidth, "col-width", 60, "specifies the max column width of output")
	f.BoolVar(&o.allNamespaces, "all-namespaces", false, "list releases across all namespaces")

	return cmd
}

func (o *listOptions) run(out io.Writer) error {
	sortBy := hapi.SortByName
	if o.byDate {
		sortBy = hapi.SortByLastReleased
	}

	sortOrder := hapi.SortAsc
	if o.sortDesc {
		sortOrder = hapi.SortDesc
	}

	stats := o.statusCodes()

	res, err := o.client.ListReleases(
		helm.ReleaseListLimit(o.limit),
		helm.ReleaseListOffset(o.offset),
		helm.ReleaseListFilter(o.filter),
		helm.ReleaseListSort(sortBy),
		helm.ReleaseListOrder(sortOrder),
		helm.ReleaseListStatuses(stats),
	)

	if err != nil {
		return err
	}

	if len(res) == 0 {
		return nil
	}

	rels := filterList(res)

	if o.short {
		for _, r := range rels {
			fmt.Fprintln(out, r.Name)
		}
		return nil
	}
	fmt.Fprintln(out, formatList(rels, o.colWidth))
	return nil
}

// filterList returns a list scrubbed of old releases.
func filterList(rels []*release.Release) []*release.Release {
	idx := map[string]int{}

	for _, r := range rels {
		name, version := r.Name, r.Version
		if max, ok := idx[name]; ok {
			// check if we have a greater version already
			if max > version {
				continue
			}
		}
		idx[name] = version
	}

	uniq := make([]*release.Release, 0, len(idx))
	for _, r := range rels {
		if idx[r.Name] == r.Version {
			uniq = append(uniq, r)
		}
	}
	return uniq
}

// statusCodes gets the list of status codes that are to be included in the results.
func (o *listOptions) statusCodes() []release.ReleaseStatus {
	if o.all {
		return []release.ReleaseStatus{
			release.StatusUnknown,
			release.StatusDeployed,
			release.StatusDeleted,
			release.StatusDeleting,
			release.StatusFailed,
			release.StatusPendingInstall,
			release.StatusPendingUpgrade,
			release.StatusPendingRollback,
		}
	}
	status := []release.ReleaseStatus{}
	if o.deployed {
		status = append(status, release.StatusDeployed)
	}
	if o.deleted {
		status = append(status, release.StatusDeleted)
	}
	if o.deleting {
		status = append(status, release.StatusDeleting)
	}
	if o.failed {
		status = append(status, release.StatusFailed)
	}
	if o.superseded {
		status = append(status, release.StatusSuperseded)
	}
	if o.pending {
		status = append(status, release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback)
	}

	// Default case.
	if len(status) == 0 {
		status = append(status, release.StatusDeployed, release.StatusFailed)
	}
	return status
}

func formatList(rels []*release.Release, colWidth uint) string {
	table := uitable.New()

	table.MaxColWidth = colWidth
	table.AddRow("NAME", "REVISION", "UPDATED", "STATUS", "CHART", "NAMESPACE")
	for _, r := range rels {
		md := r.Chart.Metadata
		c := fmt.Sprintf("%s-%s", md.Name, md.Version)
		t := "-"
		if tspb := r.Info.LastDeployed; !tspb.IsZero() {
			t = tspb.String()
		}
		s := r.Info.Status.String()
		v := r.Version
		n := r.Namespace
		table.AddRow(r.Name, v, t, s, c, n)
	}
	return table.String()
}
