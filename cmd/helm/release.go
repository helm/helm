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
	addCommands(releaseCmd())
}

func releaseCmd() cli.Command {
	return cli.Command{
		Name:      "release",
		Usage:     "Release a chart to a remote chart repository.",
		ArgsUsage: "PATH",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "destination,u",
				Usage: "Destination URL to which this will be POSTed.",
			},
		},
	}
}
