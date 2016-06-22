/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/timeconv"
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

var allValues = false

var errReleaseRequired = errors.New("release name is required")

var getCommand = &cobra.Command{
	Use:               "get [flags] RELEASE_NAME",
	Short:             "download a named release",
	Long:              getHelp,
	RunE:              getCmd,
	PersistentPreRunE: setupConnection,
}

var getValuesCommand = &cobra.Command{
	Use:   "values [flags] RELEASE_NAME",
	Short: "download the values file for a named release",
	Long:  getValuesHelp,
	RunE:  getValues,
}

var getManifestCommand = &cobra.Command{
	Use:   "manifest [flags] RELEASE_NAME",
	Short: "download the manifest for a named release",
	Long:  getManifestHelp,
	RunE:  getManifest,
}

func init() {
	// 'get' command flags.
	getCommand.PersistentFlags().StringVarP(&getOut, "file", "f", "", "output file")

	// 'get values' flags.
	getValuesCommand.PersistentFlags().BoolVarP(&allValues, "all", "a", false, "dump all (computed) values")

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
		return prettyError(err)
	}

	cfg, err := chartutil.CoalesceValues(res.Release.Chart, res.Release.Config, nil)
	if err != nil {
		return err
	}
	cfgStr, err := cfg.YAML()
	if err != nil {
		return err
	}

	fmt.Printf("CHART: %s-%s\n", res.Release.Chart.Metadata.Name, res.Release.Chart.Metadata.Version)
	fmt.Printf("RELEASED: %s\n", timeconv.Format(res.Release.Info.LastDeployed, time.ANSIC))
	fmt.Println("USER-SUPPLIED VALUES:")
	fmt.Println(res.Release.Config.Raw)
	fmt.Println("COMPUTED VALUES:")
	fmt.Println(cfgStr)
	fmt.Println("MANIFEST:")
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
		return prettyError(err)
	}

	// If the user wants all values, compute the values and return.
	if allValues {
		cfg, err := chartutil.CoalesceValues(res.Release.Chart, res.Release.Config, nil)
		if err != nil {
			return err
		}
		cfgStr, err := cfg.YAML()
		if err != nil {
			return err
		}
		return getToFile(cfgStr)
	}

	return getToFile(res.Release.Config.Raw)
}

// getManifest implements 'helm get manifest'
func getManifest(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseContent(args[0])
	if err != nil {
		return prettyError(err)
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
