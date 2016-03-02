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
	"github.com/kubernetes/deployment-manager/pkg/format"
)

func init() {
	addCommands(deleteCmd())
}

func deleteCmd() cli.Command {
	return cli.Command{
		Name:      "delete",
		Usage:     "Deletes the supplied deployment",
		ArgsUsage: "DEPLOYMENT",
		Action:    func(c *cli.Context) { run(c, deleteDeployment) },
	}
}

func deleteDeployment(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("First argument, deployment name, is required. Try 'helm get --help'")
	}
	name := args[0]
	deployment, err := client(c).DeleteDeployment(name)
	if err != nil {
		return err
	}
	return format.YAML(deployment)
}
