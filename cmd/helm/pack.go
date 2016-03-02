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

	"github.com/codegangsta/cli"
	"github.com/kubernetes/deployment-manager/pkg/chart"
	"github.com/kubernetes/deployment-manager/pkg/format"
)

func init() {
	addCommands(packageCmd())
}

func packageCmd() cli.Command {
	return cli.Command{
		Name:      "package",
		Aliases:   []string{"pack"},
		Usage:     "Given a chart directory, package it into a release.",
		ArgsUsage: "PATH",
		Action:    func(c *cli.Context) { run(c, pack) },
	}
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

	c, err := chart.LoadDir(dir)
	if err != nil {
		return fmt.Errorf("Failed to load %s: %s", dir, err)
	}

	fname, err := chart.Save(c, ".")
	format.Msg(fname)
	return nil
}
