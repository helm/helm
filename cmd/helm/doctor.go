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
	"github.com/kubernetes/deployment-manager/pkg/dm"
	"github.com/kubernetes/deployment-manager/pkg/format"
	"github.com/kubernetes/deployment-manager/pkg/kubectl"
)

func init() {
	addCommands(doctorCmd())
}

func doctorCmd() cli.Command {
	return cli.Command{
		Name:      "doctor",
		Usage:     "Run a series of checks for necessary prerequisites.",
		ArgsUsage: "",
		Action:    func(c *cli.Context) { run(c, doctor) },
	}
}

func doctor(c *cli.Context) error {
	var runner kubectl.Runner
	runner = &kubectl.RealRunner{}
	if dm.IsInstalled(runner) {
		format.Success("You have everything you need. Go forth my friend!")
	} else {
		format.Warning("Looks like you don't have DM installed.\nRun: `helm install`")
	}

	return nil
}
