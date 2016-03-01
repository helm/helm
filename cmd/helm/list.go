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
	"github.com/kubernetes/deployment-manager/pkg/format"
)

func init() {
	addCommands(listCmd())
}

func listCmd() cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "Lists the deployments in the cluster",
		Action: func(c *cli.Context) { run(c, list) },
	}
}

func list(c *cli.Context) error {
	list, err := client(c).ListDeployments()
	if err != nil {
		return err
	}
	return format.YAML(list)
}
