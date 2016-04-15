package main

import (
	"os"
	"path/filepath"

	"github.com/deis/tiller/pkg/chart"
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

func init() {
	RootCommand.AddCommand(packageCmd)
}

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Package a chart directory into a chart archive.",
	Long:  packageDesc,
	RunE:  runPackage,
}

func runPackage(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	ch, err := chart.LoadDir(path)
	if err != nil {
		return err
	}

	// Save to the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name, err := chart.Save(ch, cwd)
	if err == nil {
		cmd.Printf("Saved %s", name)
	}
	return err
}
