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
	"io"
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
`

var getManifestHelp = `
This command fetches the generated manifest for a given release.

A manifest is a YAML-encoded representation of the Kubernetes resources that
were generated from this release's chart(s). If a chart is dependent on other
charts, those resources will also be included in the manifest.
`

var errReleaseRequired = errors.New("release name is required")

type getCmd struct {
	release string
	out     io.Writer
	client  helm.Interface
}

func newGetCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:   "get [flags] RELEASE_NAME",
		Short: "download a named release",
		Long:  getHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			return get.run()
		},
		PersistentPreRunE: setupConnection,
	}
	cmd.AddCommand(newGetValuesCmd(client, out))
	cmd.AddCommand(newGetManifestCmd(client, out))
	return cmd
}

type getValuesCmd struct {
	allValues bool
	getCmd
}

func newGetValuesCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getValuesCmd{}
	get.out = out
	get.client = client
	cmd := &cobra.Command{
		Use:   "values [flags] RELEASE_NAME",
		Short: "download the values file for a named release",
		Long:  getValuesHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			return get.run()
		},
	}
	cmd.Flags().BoolVarP(&get.allValues, "all", "a", false, "dump all (computed) values")
	return cmd
}

type getManifestCmd struct {
	getCmd
}

func newGetManifestCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getManifestCmd{}
	get.out = out
	get.client = client
	cmd := &cobra.Command{
		Use:   "manifest [flags] RELEASE_NAME",
		Short: "download the manifest for a named release",
		Long:  getManifestHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			return get.run()
		},
	}
	return cmd
}

// getCmd is the command that implements 'helm get'
func (g *getCmd) run() error {
	res, err := helm.GetReleaseContent(g.release)
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

	fmt.Fprintf(g.out, "CHART: %s-%s\n", res.Release.Chart.Metadata.Name, res.Release.Chart.Metadata.Version)
	fmt.Fprintf(g.out, "RELEASED: %s\n", timeconv.Format(res.Release.Info.LastDeployed, time.ANSIC))
	fmt.Fprintln(g.out, "USER-SUPPLIED VALUES:")
	fmt.Fprintln(g.out, res.Release.Config.Raw)
	fmt.Fprintln(g.out, "COMPUTED VALUES:")
	fmt.Fprintln(g.out, cfgStr)
	fmt.Fprintln(g.out, "MANIFEST:")
	fmt.Fprintln(g.out, res.Release.Manifest)
	return nil
}

// getValues implements 'helm get values'
func (g *getValuesCmd) run() error {
	res, err := helm.GetReleaseContent(g.release)
	if err != nil {
		return prettyError(err)
	}

	// If the user wants all values, compute the values and return.
	if g.allValues {
		cfg, err := chartutil.CoalesceValues(res.Release.Chart, res.Release.Config, nil)
		if err != nil {
			return err
		}
		cfgStr, err := cfg.YAML()
		if err != nil {
			return err
		}
		fmt.Fprintln(g.out, cfgStr)
		return nil
	}

	fmt.Fprintln(g.out, res.Release.Config.Raw)
	return nil
}

// getManifest implements 'helm get manifest'
func (g *getManifestCmd) run() error {
	res, err := helm.GetReleaseContent(g.release)
	if err != nil {
		return prettyError(err)
	}
	fmt.Fprintln(g.out, res.Release.Manifest)
	return nil
}
