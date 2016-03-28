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
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
	"github.com/kubernetes/helm/pkg/repo"
)

func init() {
	addCommands(repoCommands())
}

const chartRepoPath = "repositories"

const repoDesc = `Helm repositories store Helm charts.

   The repository commands are used to manage which Helm repositories Helm may
   use as a source for Charts. The repositories are accessed by in-cluster Helm
   components.

   To list the repositories that your server knows about, use 'helm repo list'.

   For more details, use 'helm repo CMD -h'.
`

const addRepoDesc = `The add repository command is used to add a name a repository url to your
   chart repository list. The repository url must begin with a valid protocoal
   These include https, http, and gs.

   A valid command might look like:
   $ helm repo add charts gs://kubernetes-charts
`

func repoCommands() cli.Command {
	return cli.Command{
		Name:        "repository",
		Aliases:     []string{"repo"},
		Usage:       "Perform chart repository operations.",
		Description: repoDesc,
		Subcommands: []cli.Command{
			{
				Name:        "add",
				Usage:       "Add a chart repository to the remote manager.",
				Description: addRepoDesc,
				ArgsUsage:   "[NAME] [REPOSITORY_URL]",
				Action:      func(c *cli.Context) { run(c, addRepo) },
			},
			{
				Name:      "list",
				Usage:     "List the chart repositories on the remote manager.",
				ArgsUsage: "",
				Action:    func(c *cli.Context) { run(c, listRepos) },
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Remove a chart repository from the remote manager.",
				ArgsUsage: "REPOSITORY_NAME",
				Action:    func(c *cli.Context) { run(c, removeRepo) },
			},
		},
	}
}

func addRepo(c *cli.Context) error {
	args := c.Args()
	if len(args) < 2 {
		return errors.New("'helm repo add' requires a name and repository url as arguments")
	}
	name := args[0]
	repoURL := args[1]
	payload, _ := json.Marshal(repo.Repo{URL: repoURL, Name: name})
	msg := ""
	if _, err := NewClient(c).Post(chartRepoPath, payload, &msg); err != nil {
		return err
	}
	format.Info(name + " has been added to your chart repositories!")
	return nil
}

func listRepos(c *cli.Context) error {
	dest := map[string]string{}
	if _, err := NewClient(c).Get(chartRepoPath, &dest); err != nil {
		return err
	}
	if len(dest) < 1 {
		format.Info("Looks like you don't have any chart repositories.")
		format.Info("Add a chart repository using the `helm repo add [REPOSITORY_URL]` command.")
	} else {
		format.Msg("Chart Repositories:\n")
		for k, v := range dest {
			//TODO: make formatting pretty
			format.Msg(k + "\t" + v + "\n")
		}
	}
	return nil
}

func removeRepo(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("'helm repo remove' requires a repository name as an argument")
	}
	name := args[0]
	if _, err := NewClient(c).Delete(filepath.Join(chartRepoPath, name), nil); err != nil {
		return err
	}
	format.Msg(name + " has been removed.\n")
	return nil
}
