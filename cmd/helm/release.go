package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(releaseCmd())
}

func releaseCmd() cli.Command {
	return cli.Command{
		Name:      "release",
		Usage:     "Release a chart to a remote chart repository.",
		ArgsUsage: "PATH",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "destination,u",
				Usage: "Destination URL to which this will be POSTed.",
			},
		},
	}
}
