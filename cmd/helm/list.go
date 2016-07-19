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

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/timeconv"
)

var listHelp = `
This command lists all of the currently deployed releases.

By default, items are sorted alphabetically. Use the '-d' flag to sort by
release date.

If an argument is provided, it will be treated as a filter. Filters are
regular expressions (Perl compatible) that are applied to the list of releases.
Only items that match the filter will be returned.

	$ helm list -l 'ara[a-z]+'
	NAME            	UPDATED                 	CHART
	maudlin-arachnid	Mon May  9 16:07:08 2016	alpine-0.1.0

If no results are found, 'helm list' will exit 0, but with no output (or in
the case of '-l', only headers).

By default, up to 256 items may be returned. To limit this, use the '--max' flag.
Setting '--max' to 0 will not return all results. Rather, it will return the
server's default, which may be much higher than 256. Pairing the '--max'
flag with the '--offset' flag allows you to page through results.
`

type listCmd struct {
	filter   string
	long     bool
	limit    int
	offset   string
	byDate   bool
	sortDesc bool
	out      io.Writer
	client   helm.Interface
}

func newListCmd(client helm.Interface, out io.Writer) *cobra.Command {
	list := &listCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:               "list [flags] [FILTER]",
		Short:             "list releases",
		Long:              listHelp,
		Aliases:           []string{"ls"},
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				list.filter = strings.Join(args, " ")
			}
			if list.client == nil {
				list.client = helm.NewClient(helm.Host(tillerHost))
			}
			return list.run()
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&list.long, "long", "l", false, "output long listing format")
	f.BoolVarP(&list.byDate, "date", "d", false, "sort by release date")
	f.BoolVarP(&list.sortDesc, "reverse", "r", false, "reverse the sort order")
	f.IntVarP(&list.limit, "max", "m", 256, "maximum number of releases to fetch")
	f.StringVarP(&list.offset, "offset", "o", "", "the next release name in the list, used to offset from start value")
	return cmd
}

func (l *listCmd) run() error {
	sortBy := services.ListSort_NAME
	if l.byDate {
		sortBy = services.ListSort_LAST_RELEASED
	}

	sortOrder := services.ListSort_ASC
	if l.sortDesc {
		sortOrder = services.ListSort_DESC
	}

	res, err := l.client.ListReleases(
		helm.ReleaseListLimit(l.limit),
		helm.ReleaseListOffset(l.offset),
		helm.ReleaseListFilter(l.filter),
		helm.ReleaseListSort(int32(sortBy)),
		helm.ReleaseListOrder(int32(sortOrder)),
	)

	if err != nil {
		return prettyError(err)
	}

	if len(res.Releases) == 0 {
		return nil
	}

	if res.Next != "" {
		fmt.Fprintf(l.out, "\tnext: %s", res.Next)
	}

	rels := res.Releases

	if l.long {
		fmt.Fprintln(l.out, formatList(rels))
		return nil
	}
	for _, r := range rels {
		fmt.Fprintln(l.out, r.Name)
	}

	return nil
}

func formatList(rels []*release.Release) string {
	table := uitable.New()
	table.MaxColWidth = 30
	table.AddRow("NAME", "VERSION", "UPDATED", "STATUS", "CHART")
	for _, r := range rels {
		c := fmt.Sprintf("%s-%s", r.Chart.Metadata.Name, r.Chart.Metadata.Version)
		t := timeconv.String(r.Info.LastDeployed)
		s := r.Info.Status.Code.String()
		v := r.Version
		table.AddRow(r.Name, v, t, s, c)
	}
	return table.String()
}
