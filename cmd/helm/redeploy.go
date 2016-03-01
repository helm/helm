package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(redeployCommand())
}

func redeployCommand() cli.Command {
	return cli.Command{
		Name:      "redeploy",
		Usage:     "update an existing deployment with a new configuration.",
		ArgsUsage: "DEPLOYMENT",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config,f",
				Usage: "Configuration values file.",
			},
		},
	}
}
