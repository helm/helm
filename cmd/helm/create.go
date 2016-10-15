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
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const createDesc = `
This command creates a chart directory along with the common files and
directories used in a chart.

For example, 'helm create foo' will create a directory structure that looks
something like this:

	foo/
	  |
	  |- .helmignore   # Contains patterns to ignore when packaging Helm charts.
	  |
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

type createCmd struct {
	name string
	out  io.Writer
}

func newCreateCmd(out io.Writer) *cobra.Command {
	cc := &createCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "create a new chart with the given name",
		Long:  createDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("the name of the new chart is required")
			}
			cc.name = args[0]
			return cc.run()
		},
	}

	return cmd
}

func (c *createCmd) run() error {
	fmt.Fprintf(c.out, "Creating %s\n", c.name)

	chartname := filepath.Base(c.name)
	cfile := &chart.Metadata{
		Name:        chartname,
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.0",
		ApiVersion:  chartutil.ApiVersionV1,
	}

	_, err := chartutil.Create(cfile, filepath.Dir(c.name))
	return err
}
