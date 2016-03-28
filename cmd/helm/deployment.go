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
	"regexp"
	"text/template"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/format"
)

var (
	errMissingDeploymentArg = errors.New("First argument, deployment name, is required. Try 'helm get --help'")
	errTooManyArgs          = errors.New("Too many arguments provided. Try 'helm dep describe [DEPLOYMENT]'")
)

const deploymentDesc = `A deployment is an instance of a Chart running in the cluster.

   Deployments have a name, a chart, and possibly a set of properites. The deployment
   commands provide tools for managing deployments.

   To deploy a new chart, use the top-level 'helm deploy' command. From there,
   the 'helm deployment' commands may be used to work with the deployed
   application.

   For more help, use 'helm deployment CMD -h'.`

const defaultShowFormat = `Name: {{.Name}}
Status: {{.State.Status}}
{{with .State.Errors}}Errors:
{{range .}}  {{.}}{{end}}
{{end}}`

func init() {
	addCommands(deploymentCommands())
}

func deploymentCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:        "deployment",
		Aliases:     []string{"dep"},
		Usage:       "Perform deployment-centered operations.",
		Description: deploymentDesc,
		Subcommands: []cli.Command{
			{
				Name:      "config",
				Usage:     "Dump the configuration file for this deployment.",
				ArgsUsage: "DEPLOYMENT",
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Deletes the named deployment(s).",
				ArgsUsage: "DEPLOYMENT [DEPLOYMENT [...]]",
				Action:    func(c *cli.Context) { run(c, deleteDeployment) },
			},
			{
				Name:      "describe",
				Usage:     "Describes the kubernetes resources for the named deployment(s).",
				ArgsUsage: "DEPLOYMENT",
				Action:    func(c *cli.Context) { run(c, describeDeployment) },
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
				Action:    func(c *cli.Context) { run(c, showDeployment) },
			},
			{
				Name:      "list",
				Aliases:   []string{"ls"},
				Usage:     "list all deployments, or filter by an optional regular expression.",
				ArgsUsage: "REGEXP",
				Action:    func(c *cli.Context) { run(c, listDeployments) },
			},
		},
	}
}

func listDeployments(c *cli.Context) error {
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

func deleteDeployment(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errMissingDeploymentArg
	}
	for _, name := range args {
		deployment, err := NewClient(c).DeleteDeployment(name)
		if err != nil {
			return err
		}
		format.Info("Deleted %q at %s", name, deployment.DeletedAt)
	}
	return nil
}

func describeDeployment(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errMissingDeploymentArg
	}
	if len(args) > 1 {
		return errTooManyArgs
	}
	name := args[0]
	_, err := NewClient(c).DescribeDeployment(name)
	if err != nil {
		return err
	}

	format.Info("TO BE IMPLEMENTED")

	return nil
}

func showDeployment(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errMissingDeploymentArg
	}
	name := args[0]
	deployment, err := NewClient(c).GetDeployment(name)
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("show").Parse(defaultShowFormat))
	return tmpl.Execute(os.Stdout, deployment)
}
