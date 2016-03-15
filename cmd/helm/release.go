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
	"os"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
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
		Action: func(c *cli.Context) { run(c, release) },
	}
}

func release(c *cli.Context) error {
	a := c.Args()
	if len(a) == 0 {
		return errors.New("'helm release' requires a path to a chart archive or directory.")
	}

	var arch string
	if fi, err := os.Stat(a[0]); err != nil {
		return err
	} else if fi.IsDir() {
		var err error
		arch, err = packDir(a[0])
		if err != nil {
			return err
		}
	} else {
		arch = a[0]
	}

	u, err := NewClient(c).PostChart(arch, arch)
	format.Msg(u)
	return err
}
