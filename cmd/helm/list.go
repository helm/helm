package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
)

func listCmd() cli.Command {
	return cli.Command{
		Name:  "list",
		Usage: "Lists the deployments in the cluster",
		Action: func(c *cli.Context) {
			if err := list(c.GlobalString("host")); err != nil {
				format.Error("%s (Is the cluster running?)", err)
				os.Exit(1)
			}
		},
	}
}

func list(host string) error {
	client := dm.NewClient(host)
	client.Protocol = "http"
	return client.ListDeployments()
}
