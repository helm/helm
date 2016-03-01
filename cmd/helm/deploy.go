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
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/deployment-manager/pkg/common"
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
				Name:  "name",
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

	// If there is a configuration file, use it.
	cfg := &common.Configuration{}
	if c.String("config") != "" {
		if err := loadConfig(cfg, c.String("config")); err != nil {
			return err
		}
	} else {
		cfg.Resources = []*common.Resource{
			{
				Properties: map[string]interface{}{},
			},
		}
	}

	// If there is a chart specified on the commandline, override the config
	// file with it.
	args := c.Args()
	if len(args) > 0 {
		cname := args[0]
		if isLocalChart(cname) {
			// If we get here, we need to first package then upload the chart.
			loc, err := doUpload(cname, "", c)
			if err != nil {
				return err
			}
			cfg.Resources[0].Name = loc
		} else {
			cfg.Resources[0].Type = cname
		}
	}

	// Override the name if one is passed in.
	if name := c.String("name"); len(name) > 0 {
		cfg.Resources[0].Name = name
	}

	if props, err := parseProperties(c.String("properties")); err != nil {
		return err
	} else if len(props) > 0 {
		// Coalesce the properties into the first props. We have no way of
		// knowing which resource the properties are supposed to be part
		// of.
		for n, v := range props {
			cfg.Resources[0].Properties[n] = v
		}
	}

	return client(c).PostDeployment(cfg)
}

// isLocalChart returns true if the given path can be statted.
func isLocalChart(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadConfig loads a file into a common.Configuration.
func loadConfig(c *common.Configuration, filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}
