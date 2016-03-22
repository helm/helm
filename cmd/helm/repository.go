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
	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
)

func init() {
	addCommands(repoCommands())
}

const chartRepoPath = "chart_repositories"

func repoCommands() cli.Command {
	return cli.Command{
		Name:    "repository",
		Aliases: []string{"repo"},
		Usage:   "Perform repository operations.",
		Subcommands: []cli.Command{
			{
				Name:      "add",
				Usage:     "Add a repository to the remote manager.",
				ArgsUsage: "REPOSITORY",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "cred",
						Usage: "The name of the credential.",
					},
				},
				Action: func(c *cli.Context) { run(c, addRepo) },
			},
			{
				Name:      "show",
				Usage:     "Show the repository details for a given repository.",
				ArgsUsage: "REPOSITORY",
			},
			{
				Name:      "list",
				Usage:     "List the repositories on the remote manager.",
				ArgsUsage: "",
				Action:    func(c *cli.Context) { run(c, listRepos) },
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Remove a repository from the remote manager.",
				ArgsUsage: "REPOSITORY",
				Action:    func(c *cli.Context) { run(c, removeRepo) },
			},
		},
	}
}

func addRepo(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("'helm repo add' requires a repository as an argument")
	}
	dest := ""
	if _, err := NewClient(c).Post(chartRepoPath, nil, &dest); err != nil {
		return err
	}
	format.Msg(dest)
	return nil
}

func listRepos(c *cli.Context) error {
	dest := ""
	if _, err := NewClient(c).Get(chartRepoPath, &dest); err != nil {
		return err
	}
	format.Msg(dest)
	return nil
}

func removeRepo(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("'helm repo remove' requires a repository as an argument")
	}
	dest := ""
	if _, err := NewClient(c).Delete(chartRepoPath, &dest); err != nil {
		return err
	}
	format.Msg(dest)
	return nil
}
