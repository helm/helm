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
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/chart"
)

func init() {
	addCommands(chartCommands())
}

func chartCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:  "chart",
		Usage: "Perform chart-centered operations.",
		Subcommands: []cli.Command{
			{
				Name:      "config",
				Usage:     "Create a configuration parameters file for this chart.",
				ArgsUsage: "CHART",
			},
			{
				Name:      "show",
				Aliases:   []string{"info"},
				Usage:     "Provide details about this package.",
				ArgsUsage: "CHART",
			},
			{
				Name:      "create",
				Usage:     "Create a new chart directory and set up base files and directories.",
				ArgsUsage: "CHARTNAME",
				Action:    func(c *cli.Context) { run(c, createChart) },
			},
			{
				Name:      "list",
				Usage:     "list all deployed charts, optionally constraining by pattern.",
				ArgsUsage: "[PATTERN]",
			},
			{
				Name:      "deployments",
				Usage:     "given a chart, show all the deployments that reference it.",
				ArgsUsage: "CHART",
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
