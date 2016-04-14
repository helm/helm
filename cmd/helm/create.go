package main

import (
	"errors"
	"os/user"
	"path/filepath"

	"github.com/deis/tiller/pkg/chart"
	"github.com/spf13/cobra"
)

const createDesc = `
This command creates a chart directory along with the common files and
directories used in a chart.

For example, 'helm create foo' will create a directory structure that looks
something like this:

	foo/
	  |- Chart.yaml
	  |
	  |- values.toml
	  |
	  |- templates/

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
		Maintainers: []*chart.Maintainer{
			{Name: username()},
		},
	}

	if _, err := chart.Create(&cfile, filepath.Dir(cname)); err != nil {
		return err
	}

	return nil
}

func username() string {
	uname := "Unknown"
	u, err := user.Current()
	if err == nil {
		uname = u.Name
		if uname == "" {
			uname = u.Username
		}
	}
	return uname
}
