package main

import (
	"fmt"
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
				format.Err("%s (Is the cluster running?)", err)
				os.Exit(1)
			}
		},
	}
}

func list(host string) error {
	client := dm.NewClient(host).SetDebug(isDebugging)
	list, err := client.ListDeployments()
	if err != nil {
		return err
	}
	fmt.Println(list)
	return nil
}
