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
	addCommands(credCommands())
}

func credCommands() cli.Command {
	return cli.Command{
		Name:    "credential",
		Aliases: []string{"cred"},
		Usage:   "Perform repository credential operations.",
		Subcommands: []cli.Command{
			{
				Name:  "add",
				Usage: "Add a credential to the remote manager.",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "file,f",
						Usage: "A JSON file with credential information.",
					},
				},
				ArgsUsage: "CREDENTIAL",
			},
			{
				Name:      "list",
				Usage:     "List the credentials on the remote manager.",
				ArgsUsage: "",
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Remove a credential from the remote manager.",
				ArgsUsage: "CREDENTIAL",
			},
		},
	}
}
