package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/gosuri/uitable"
	"github.com/kubernetes/helm/pkg/helm"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
	"github.com/kubernetes/helm/pkg/timeconv"
	"github.com/spf13/cobra"
)

var listHelp = `
This command lists all of the currently deployed releases.

By default, items are sorted alphabetically. Sorting is done client-side, so if
the number of releases is less than the setting in '--max', some values will
be omitted, and in no particular lexicographic order.
`

var listCommand = &cobra.Command{
	Use:     "list [flags]",
	Short:   "List releases",
	Long:    listHelp,
	RunE:    listCmd,
	Aliases: []string{"ls"},
}

var listLong bool
var listMax int
var listOffset int
var listByDate bool

func init() {
	listCommand.Flags().BoolVarP(&listLong, "long", "l", false, "output long listing format")
	listCommand.Flags().BoolVarP(&listByDate, "date", "d", false, "sort by release date")
	listCommand.Flags().IntVarP(&listMax, "max", "m", 256, "maximum number of releases to fetch")
	listCommand.Flags().IntVarP(&listOffset, "offset", "o", 0, "offset from start value (zero-indexed)")
	RootCommand.AddCommand(listCommand)
}

func listCmd(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		fmt.Println("TODO: Implement filter.")
	}

	res, err := helm.ListReleases(listMax, listOffset)
	if err != nil {
		return err
	}

	rels := res.Releases
	if res.Count+res.Offset < res.Total {
		fmt.Println("Not all values were fetched.")
	}

	if listByDate {
		sort.Sort(byDate(rels))
	} else {
		sort.Sort(byName(rels))
	}

	// Purty output, ya'll
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
	table.AddRow("NAME", "UPDATED", "CHART")
	for _, r := range rels {
		c := fmt.Sprintf("%s-%s", r.Chart.Metadata.Name, r.Chart.Metadata.Version)
		t := timeconv.Format(r.Info.LastDeployed, time.ANSIC)
		table.AddRow(r.Name, t, c)
	}
	fmt.Println(table)

	return nil
}

// byName implements the sort.Interface for []*release.Release.
type byName []*release.Release

func (r byName) Len() int {
	return len(r)
}
func (r byName) Swap(p, q int) {
	r[p], r[q] = r[q], r[p]
}
func (r byName) Less(i, j int) bool {
	return r[i].Name < r[j].Name
}

type byDate []*release.Release

func (r byDate) Len() int { return len(r) }
func (r byDate) Swap(p, q int) {
	r[p], r[q] = r[q], r[p]
}
func (r byDate) Less(p, q int) bool {
	return r[p].Info.LastDeployed.Seconds < r[q].Info.LastDeployed.Seconds
}
