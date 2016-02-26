package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(deploymentCommands())
}

func deploymentCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:  "deployment",
		Usage: "Perform deployment-centered operations.",
		Subcommands: []cli.Command{
			{
				Name:      "config",
				Usage:     "Dump the configuration file for this deployment.",
				ArgsUsage: "DEPLOYMENT",
			},
			{
				Name:      "manifest",
				Usage:     "Dump the Kubernetes manifest file for this deployment.",
				ArgsUsage: "DEPLOYMENT",
			},
			{
				Name:      "show",
				Aliases:   []string{"info"},
				Usage:     "Provide details about this deployment.",
				ArgsUsage: "",
			},
			{
				Name:      "list",
				Usage:     "list all deployments, or filter by an optional pattern",
				ArgsUsage: "PATTERN",
			},
		},
	}
}
