package main

import (
	"fmt"

	"github.com/kubernetes/helm/pkg/lint"
	"github.com/spf13/cobra"
)

var longLintHelp = `
This command takes a path to a chart and runs a series of tests to verify that
the chart is well-formed.

If the linter encounters things that will cause the chart to fail installation,
it will emit [ERROR] messages. If it encounters issues that break with convention
or recommendation, it will emit [WARNING] messages.
`

var lintCommand = &cobra.Command{
	Use:   "lint [flags] PATH",
	Short: "Examines a chart for possible issues",
	Long:  longLintHelp,
	Run:   lintCmd,
}

func init() {
	RootCommand.AddCommand(lintCommand)
}

func lintCmd(cmd *cobra.Command, args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	issues := lint.All(path)
	for _, i := range issues {
		fmt.Printf("%s\n", i)
	}
}
