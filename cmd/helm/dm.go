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
	"github.com/kubernetes/deployment-manager/pkg/dm"
	"github.com/kubernetes/deployment-manager/pkg/format"
	"github.com/kubernetes/deployment-manager/pkg/kubectl"
)

// ErrAlreadyInstalled indicates that DM is already installed.
var ErrAlreadyInstalled = errors.New("Already Installed")

func init() {
	addCommands(dmCmd())
}

func dmCmd() cli.Command {
	return cli.Command{
		Name:  "dm",
		Usage: "Manage DM on Kubernetes",
		Subcommands: []cli.Command{
			{
				Name:        "install",
				Usage:       "Install DM on Kubernetes.",
				ArgsUsage:   "",
				Description: ``,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be installed, but don't install anything.",
					},
				},
				Action: func(c *cli.Context) {
					if err := install(c.Bool("dry-run")); err != nil {
						format.Err("%s (Run 'helm doctor' for more information)", err)
						os.Exit(1)
					}
				},
			},
			{
				Name:        "uninstall",
				Usage:       "Uninstall the DM from Kubernetes.",
				ArgsUsage:   "",
				Description: ``,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be installed, but don't install anything.",
					},
				},
				Action: func(c *cli.Context) {
					if err := uninstall(c.Bool("dry-run")); err != nil {
						format.Err("%s (Run 'helm doctor' for more information)", err)
						os.Exit(1)
					}
				},
			},
			{
				Name:      "status",
				Usage:     "Show status of DM.",
				ArgsUsage: "",
				Action: func(c *cli.Context) {
					format.Err("Not yet implemented")
					os.Exit(1)
				},
			},
			{
				Name:      "target",
				Usage:     "Displays information about cluster.",
				ArgsUsage: "",
				Action: func(c *cli.Context) {
					if err := target(c.Bool("dry-run")); err != nil {
						format.Err("%s (Is the cluster running?)", err)
						os.Exit(1)
					}
				},
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Only display the underlying kubectl commands.",
					},
				},
			},
		},
	}
}

func install(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Install(runner)
	if err != nil {
		format.Err("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func uninstall(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Uninstall(runner)
	if err != nil {
		format.Err("Error uninstalling: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func getKubectlRunner(dryRun bool) kubectl.Runner {
	if dryRun {
		return &kubectl.PrintRunner{}
	}
	return &kubectl.RealRunner{}
}
