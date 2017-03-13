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
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/downloader"
)

const dependencyCreateDesc = `
Update the requirements.yaml file within a given chart.

If no requirements.yaml exists in the chart directory, this command will create
a new requirements.yaml and add the provided dependency.
`

type dependencyCreateCmd struct {
	out        io.Writer
	name       string
	chartpath  string
	repository string
	version    string
	helmhome   helmpath.Home
}

// newDependencyCreateCmd creates a new dependency create command.
func newDependencyCreateCmd(out io.Writer) *cobra.Command {
	dcc := &dependencyCreateCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:     "create DEPENDENCY REPOSITORY [flags]",
		Aliases: []string{"create"},
		Short:   "add dependencies to the contents of requirements.yaml",
		Long:    dependencyCreateDesc,
		RunE: func(cmd *cobra.Command, args []string) error {

			dcc.name = args[0]
			dcc.repository = args[1]
			dcc.helmhome = helmpath.Home(homePath())

			return dcc.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&dcc.version, "version", "", "0.1.0", "set the version")
	f.StringVarP(&dcc.chartpath, "chartpath", "c", ".", "directory of chart to add dependency to ")

	return cmd
}

// run runs the full dependency create process.
func (d *dependencyCreateCmd) run() error {
	var err error
	d.chartpath, err = filepath.Abs(d.chartpath)
	if err != nil {
		return err
	}

	man := &downloader.Manager{
		Out:       d.out,
		ChartPath: d.chartpath,
		HelmHome:  d.helmhome,
	}

	return man.Create(d.name, d.repository, d.version)
}
