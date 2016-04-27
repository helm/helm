package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deis/tiller/pkg/chart"
	"github.com/deis/tiller/pkg/repo"
	"github.com/spf13/cobra"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

If no path is given, this will look in the present working directory for a
Chart.yaml file, and (if found) build the current directory into a chart.

Versioned chart archives are used by Helm package repositories.
`

var save bool

func init() {
	packageCmd.Flags().BoolVar(&save, "save", true, "save packaged chart to local chart repository")
	RootCommand.AddCommand(packageCmd)
}

var packageCmd = &cobra.Command{
	Use:   "package [CHART_PATH]",
	Short: "Package a chart directory into a chart archive.",
	Long:  packageDesc,
	RunE:  runPackage,
}

func runPackage(cmd *cobra.Command, args []string) error {
	path := "."

	if len(args) > 0 {
		path = args[0]
	} else {
		return fmt.Errorf("This command needs at least one argument, the path to the chart.")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	ch, err := chart.LoadDir(path)
	if err != nil {
		return err
	}

	// Save to $HELM_HOME/local directory.
	if save {
		if err := repo.AddChartToLocalRepo(ch, localRepoPath); err != nil {
			return err
		}
	}

	// Save to the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name, err := chart.Save(ch, cwd)
	if err == nil {
		cmd.Printf("Saved %s to current directory\n", name)
	}
	return err
}
