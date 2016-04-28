package main

import (
	"errors"
	"fmt"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/spf13/cobra"
)

var getHelp = `
This command shows the details of a named release.

It can be used to get extended information about the release, including:

  - The values used to generate the release
  - The chart used to generate the release
  - The generated manifest file

By default, this prints a human readable collection of information about the
chart, the supplied values, and the generated manifest file.
`

var errReleaseRequired = errors.New("release name is required")

var getCommand = &cobra.Command{
	Use:   "get [flags] RELEASE_NAME",
	Short: "Download a named release",
	Long:  getHelp,
	RunE:  getCmd,
}

func init() {
	RootCommand.AddCommand(getCommand)
}

func getCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseContent(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Chart/Version: %s %s\n", res.Release.Chart.Metadata.Name, res.Release.Chart.Metadata.Version)
	fmt.Println("Config:")
	fmt.Println(res.Release.Config)
	fmt.Println("\nManifest:")
	fmt.Println(res.Release.Manifest)
	return nil
}
