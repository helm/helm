package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/lint"
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
	Short: "examines a chart for possible issues",
	Long:  longLintHelp,
	RunE:  lintCmd,
}

func init() {
	RootCommand.AddCommand(lintCommand)
}

var errLintNoChart = errors.New("no chart found for linting (missing Chart.yaml)")

func lintCmd(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Guard: Error out of this is not a chart.
	if _, err := os.Stat(filepath.Join(path, "Chart.yaml")); err != nil {
		return errLintNoChart
	}

	issues := lint.All(path)

	if len(issues) == 0 {
		fmt.Println("Lint OK")
	}

	for _, i := range issues {
		fmt.Printf("%s\n", i)
	}
	return nil
}
