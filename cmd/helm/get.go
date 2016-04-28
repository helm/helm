package main

import (
	"errors"
	"fmt"
	"os"

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

var getValuesHelp = `
This command downloads a values file for a given release.

To save the output to a file, use the -f flag.
`

var getManifestHelp = `
This command fetches the generated manifest for a given release.

A manifest is a YAML-encoded representation of the Kubernetes resources that
were generated from this release's chart(s). If a chart is dependent on other
charts, those resources will also be included in the manifest.
`

// getOut is the filename to direct output.
//
// If it is blank, output is sent to os.Stdout.
var getOut = ""

var errReleaseRequired = errors.New("release name is required")

var getCommand = &cobra.Command{
	Use:   "get [flags] RELEASE_NAME",
	Short: "Download a named release",
	Long:  getHelp,
	RunE:  getCmd,
}

var getValuesCommand = &cobra.Command{
	Use:   "values [flags] RELEASE_NAME",
	Short: "Download the values file for a named release",
	Long:  getValuesHelp,
	RunE:  getValues,
}

var getManifestCommand = &cobra.Command{
	Use:   "manifest [flags] RELEASE_NAME",
	Short: "Download the manifest for a named release",
	Long:  getManifestHelp,
	RunE:  getManifest,
}

func init() {
	getCommand.PersistentFlags().StringVarP(&getOut, "file", "f", "", "output file")
	getCommand.AddCommand(getValuesCommand)
	getCommand.AddCommand(getManifestCommand)
	RootCommand.AddCommand(getCommand)
}

// getCmd is the command that implements 'helm get'
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

// getValues implements 'helm get values'
func getValues(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseContent(args[0])
	if err != nil {
		return err
	}
	return getToFile(res.Release.Config)
}

// getManifest implements 'helm get manifest'
func getManifest(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseContent(args[0])
	if err != nil {
		return err
	}
	return getToFile(res.Release.Manifest)
}

func getToFile(v interface{}) error {
	out := os.Stdout
	if len(getOut) > 0 {
		t, err := os.Create(getOut)
		if err != nil {
			return fmt.Errorf("failed to create %s: %s", getOut, err)
		}
		defer t.Close()
		out = t
	}
	fmt.Fprintln(out, v)
	return nil
}
