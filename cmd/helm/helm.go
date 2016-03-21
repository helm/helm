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
	"os"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/client"
	"github.com/kubernetes/helm/pkg/format"
	"github.com/kubernetes/helm/pkg/version"
)

var commands []cli.Command

func init() {
	addCommands(cmds()...)
}

// debug indicates whether the process is in debug mode.
//
// This is set at app start-up time, based on the presence of the --debug
// flag.
var debug bool

func main() {
	app := cli.NewApp()
	app.Name = "helm"
	app.Version = version.Version
	app.Usage = `Deploy and manage packages.`
	app.Commands = commands

	// TODO: make better
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host,u",
			Usage:  "The URL of the DM server.",
			EnvVar: "HELM_HOST",
			Value:  "https://localhost:8000/",
		},
		cli.IntFlag{
			Name:  "timeout",
			Usage: "Time in seconds to wait for response",
			Value: 10,
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable verbose debugging output",
		},
	}
	app.Before = func(c *cli.Context) error {
		debug = c.GlobalBool("debug")
		return nil
	}
	app.Run(os.Args)
}

func cmds() []cli.Command {
	return []cli.Command{
		{
			Name: "search",
		},
	}
}

func addCommands(cmds ...cli.Command) {
	commands = append(commands, cmds...)
}

func run(c *cli.Context, f func(c *cli.Context) error) {
	if err := f(c); err != nil {
		format.Err(err)
		os.Exit(1)
	}
}

// NewClient creates a new client instance preconfigured for CLI usage.
func NewClient(c *cli.Context) *client.Client {
	host := c.GlobalString("host")
	timeout := c.GlobalInt("timeout")
	return client.NewClient(host).SetDebug(debug).SetTimeout(timeout)
}
