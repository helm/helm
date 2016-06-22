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

var listCommand = &cobra.Command{
	Use:               "list [flags] [FILTER]",
	Short:             "list releases",
	Long:              listHelp,
	RunE:              listCmd,
	Aliases:           []string{"ls"},
	PersistentPreRunE: setupConnection,
}

var (
	listLong     bool
	listMax      int
	listOffset   string
	listByDate   bool
	listSortDesc bool
)

func init() {
	f := listCommand.Flags()
	f.BoolVarP(&listLong, "long", "l", false, "output long listing format")
	f.BoolVarP(&listByDate, "date", "d", false, "sort by release date")
	f.BoolVarP(&listSortDesc, "reverse", "r", false, "reverse the sort order")
	f.IntVarP(&listMax, "max", "m", 256, "maximum number of releases to fetch")
	f.StringVarP(&listOffset, "offset", "o", "", "the next release name in the list, used to offset from start value")

	RootCommand.AddCommand(listCommand)
}

func listCmd(cmd *cobra.Command, args []string) error {
	var filter string
	if len(args) > 0 {
		filter = strings.Join(args, " ")
	}

	sortBy := services.ListSort_NAME
	if listByDate {
		sortBy = services.ListSort_LAST_RELEASED
	}

	sortOrder := services.ListSort_ASC
	if listSortDesc {
		sortOrder = services.ListSort_DESC
	}

	res, err := helm.ListReleases(listMax, listOffset, sortBy, sortOrder, filter)
	if err != nil {
		return prettyError(err)
	}

	if len(res.Releases) == 0 {
		return nil
	}

	if res.Next != "" {
		fmt.Printf("\tnext: %s", res.Next)
	}

	rels := res.Releases
	if listLong {
		return formatList(rels)
	}
	for _, r := range rels {
		fmt.Println(r.Name)
	}

	return nil
}

func formatList(rels []*release.Release) error {
	table := uitable.New()
	table.MaxColWidth = 30
	table.AddRow("NAME", "UPDATED", "STATUS", "CHART")
	for _, r := range rels {
		c := fmt.Sprintf("%s-%s", r.Chart.Metadata.Name, r.Chart.Metadata.Version)
		t := timeconv.String(r.Info.LastDeployed)
		s := r.Info.Status.Code.String()
		table.AddRow(r.Name, t, s, c)
	}
	fmt.Println(table)

	return nil
}
