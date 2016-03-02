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
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(deploymentCommands())
}

func deploymentCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:  "deployment",
		Usage: "Perform deployment-centered operations.",
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
			},
		},
	}
}
