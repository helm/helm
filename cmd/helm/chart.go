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
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/format"
)

func init() {
	addCommands(chartCommands())
}

const chartDesc = `A Chart is a package that can be installed and managed by Helm.

   The 'helm chart' subcommands provide tools for working with Helm charts. To
   get started creating your own chart, use 'helm chart create NAME'.

   For details, use 'helm chart CMD -h'.
`

func chartCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:        "chart",
		Usage:       "Perform chart-centered operations.",
		Description: chartDesc,
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create a new chart directory and set up base files and directories.",
				ArgsUsage: "CHARTNAME",
				Action:    func(c *cli.Context) { run(c, createChart) },
			},
			{
				Name:      "package",
				Aliases:   []string{"pack"},
				Usage:     "Given a chart directory, package it into a release.",
				ArgsUsage: "PATH",
				Action:    func(c *cli.Context) { run(c, pack) },
			},
		},
	}
}

func createChart(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("'helm create' requires a chart name as an argument")
	}

	dir, name := filepath.Split(args[0])

	cf := &chart.Chartfile{
		Name:        name,
		Description: "Created by Helm",
		Version:     "0.1.0",
	}

	_, err := chart.Create(cf, dir)
	return err

}

func pack(cxt *cli.Context) error {
	args := cxt.Args()
	if len(args) < 1 {
		return errors.New("'helm package' requires a path to a chart directory as an argument")
	}

	dir := args[0]
	if fi, err := os.Stat(dir); err != nil {
		return fmt.Errorf("Could not find directory %s: %s", dir, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("Not a directory: %s", dir)
	}

	fname, err := packDir(dir)
	if err != nil {
		return err
	}
	format.Msg(fname)
	return nil
}

func packDir(dir string) (string, error) {
	c, err := chart.LoadDir(dir)
	if err != nil {
		return "", fmt.Errorf("Failed to load %s: %s", dir, err)
	}

	return chart.Save(c, ".")
}
