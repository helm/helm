package main

import (
	"fmt"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
	"github.com/spf13/cobra"
)

var listHelp = `
This command lists all of the currently deployed releases.

By default, items are sorted alphabetically.
`

var listCommand = &cobra.Command{
	Use:   "list [flags] [FILTER]",
	Short: "List releases",
	Long:  listHelp,
	RunE:  listCmd,
}

var listLong bool
var listMax int

func init() {
	listCommand.LocalFlags().BoolVar(&listLong, "l", false, "output long listing format")
	listCommand.LocalFlags().IntVar(&listMax, "m", 256, "maximum number of releases to fetch")
	RootCommand.AddCommand(listCommand)
}

func listCmd(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		fmt.Println("TODO: Implement filter.")
	}

	res, err := helm.ListReleases(listMax, 0)
	if err != nil {
		return err
	}

	rels := res.Releases
	if res.Count+res.Offset < res.Total {
		fmt.Println("Not all values were fetched.")
	}

	// TODO: Add sort here.

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
	// TODO: Pretty it up
	for _, r := range rels {
		fmt.Println(r.Name)
	}

	return nil
}
