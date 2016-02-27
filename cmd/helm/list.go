package main

import (
	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/pkg/format"
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
