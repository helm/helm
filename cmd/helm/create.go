package main

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chart"
)

const createDesc = `
This command creates a chart directory along with the common files and
directories used in a chart.

For example, 'helm create foo' will create a directory structure that looks
something like this:

	foo/
	  |- Chart.yaml    # Information about your chart
	  |
	  |- values.yaml   # The default values for your templates
	  |
	  |- charts/       # Charts that this chart depends on
	  |
	  |- templates/    # The template files

'helm create' takes a path for an argument. If directories in the given path
do not exist, Helm will attempt to create them as it goes. If the given
destination exists and there are files in that directory, conflicting files
will be overwritten, but other files will be left alone.
`

func init() {
	RootCommand.AddCommand(createCmd)
}

var createCmd = &cobra.Command{
	Use:   "create [PATH]",
	Short: "Create a new chart at the location specified.",
	Long:  createDesc,
	RunE:  runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("the name of the new chart is required")
	}
	cname := args[0]
	cmd.Printf("Creating %s\n", cname)

	chartname := filepath.Base(cname)
	cfile := chart.Chartfile{
		Name:        chartname,
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.0",
	}

	_, err := chart.Create(&cfile, filepath.Dir(cname))
	return err
}
