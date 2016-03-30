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
	"fmt"
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/common"
	"gopkg.in/yaml.v2"
)

func init() {
	addCommands(deployCmd())
}

func deployCmd() cli.Command {
	return cli.Command{
		Name:      "deploy",
		Usage:     "Deploy a chart into the cluster.",
		ArgsUsage: "[CHART]",
		Action:    func(c *cli.Context) { run(c, deploy) },
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config,c",
				Usage: "The configuration YAML file for this deployment.",
			},
			cli.StringFlag{
				Name:  "name,n",
				Usage: "Name of deployment, used for deploy and update commands (defaults to template name)",
			},
			// TODO: I think there is a Generic flag type that we can implement parsing with.
			cli.StringFlag{
				Name:  "properties,p",
				Usage: "A comma-separated list of key=value pairs: 'foo=bar,foo2=baz'.",
			},
		},
	}
}

func deploy(c *cli.Context) error {

	res := &common.Resource{
		// By default
		Properties: map[string]interface{}{},
	}

	if c.String("config") != "" {
		// If there is a configuration file, use it.
		err := loadConfig(c.String("config"), &res.Properties)
		if err != nil {
			return err
		}
	}

	args := c.Args()
	if len(args) == 0 {
		return fmt.Errorf("Need chart name on commandline")
	}
	res.Type = args[0]

	if name := c.String("name"); len(name) > 0 {
		res.Name = name
	} else {
		return fmt.Errorf("Need deployed name on commandline")
	}

	if props, err := parseProperties(c.String("properties")); err != nil {
		return err
	} else if len(props) > 0 {
		// Coalesce the properties into the first props. We have no way of
		// knowing which resource the properties are supposed to be part
		// of.
		for n, v := range props {
			res.Properties[n] = v
		}
	}

	return NewClient(c).PostDeployment(res)
}

// isLocalChart returns true if the given path can be statted.
func isLocalChart(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadConfig loads chart arguments into c
func loadConfig(filename string, dest *map[string]interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, dest)
}
