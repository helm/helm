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
		Use:               "get [flags] RELEASE_NAME",
		Short:             "download a named release",
		Long:              getHelp,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			if get.client == nil {
				get.client = helm.NewClient(helm.HelmHost(helm.Config.ServAddr))
			}
			return get.run()
		},
	}
	cmd.AddCommand(newGetValuesCmd(nil, out))
	cmd.AddCommand(newGetManifestCmd(nil, out))
	return cmd
}

// getCmd is the command that implements 'helm get'
func (g *getCmd) run() error {
	res, err := g.client.ReleaseContent(g.release)
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

	fmt.Fprintf(g.out, "VERSION: %v\n", res.Release.Version)
	fmt.Fprintf(g.out, "RELEASED: %s\n", timeconv.Format(res.Release.Info.LastDeployed, time.ANSIC))
	fmt.Fprintf(g.out, "CHART: %s-%s\n", res.Release.Chart.Metadata.Name, res.Release.Chart.Metadata.Version)
	fmt.Fprintln(g.out, "USER-SUPPLIED VALUES:")
	fmt.Fprintln(g.out, res.Release.Config.Raw)
	fmt.Fprintln(g.out, "COMPUTED VALUES:")
	fmt.Fprintln(g.out, cfgStr)
	fmt.Fprintln(g.out, "MANIFEST:")
	fmt.Fprintln(g.out, res.Release.Manifest)
	return nil
}
