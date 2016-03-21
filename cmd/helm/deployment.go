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
	"regexp"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
)

func init() {
	//addCommands(listCmd())
	addCommands(deploymentCommands())
}

func deploymentCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:    "deployment",
		Aliases: []string{"dep"},
		Usage:   "Perform deployment-centered operations.",
		Subcommands: []cli.Command{
			{
				Name:      "config",
				Usage:     "Dump the configuration file for this deployment.",
				ArgsUsage: "DEPLOYMENT",
			},
			{
				Name:      "manifest",
				Usage:     "Dump the Kubernetes manifest file for this deployment.",
				ArgsUsage: "DEPLOYMENT",
			},
			{
				Name:      "show",
				Aliases:   []string{"info"},
				Usage:     "Provide details about this deployment.",
				ArgsUsage: "",
			},
			{
				Name:      "list",
				Usage:     "list all deployments, or filter by an optional pattern",
				ArgsUsage: "PATTERN",
				Action:    func(c *cli.Context) { run(c, list) },
			},
		},
	}
}

func listCmd() cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "Lists the deployments in the cluster",
		Action: func(c *cli.Context) { run(c, list) },
	}
}

func list(c *cli.Context) error {
	list, err := NewClient(c).ListDeployments()
	if err != nil {
		return err
	}
	args := c.Args()
	if len(args) >= 1 {
		pattern := args[0]
		r, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}

		newlist := []string{}
		for _, i := range list {
			if r.MatchString(i) {
				newlist = append(newlist, i)
			}
		}
		list = newlist
	}

	if len(list) == 0 {
		return errors.New("no deployments found")
	}

	format.List(list)
	return nil
}
