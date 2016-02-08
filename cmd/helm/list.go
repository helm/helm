package main

import (
	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
)

func listCmd() cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "Lists the deployments in the cluster",
		Action: func(c *cli.Context) { run(c, list) },
	}
}

func list(c *cli.Context) error {
	host := c.GlobalString("host")
	client := dm.NewClient(host).SetDebug(c.GlobalBool("debug"))
	list, err := client.ListDeployments()
	if err != nil {
		return err
	}
	return format.YAML(list)
}
