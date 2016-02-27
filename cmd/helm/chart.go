package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(chartCommands())
}

func chartCommands() cli.Command {
	return cli.Command{
		// Names following form prescribed here: http://is.gd/QUSEOF
		Name:  "chart",
		Usage: "Perform chart-centered operations.",
		Subcommands: []cli.Command{
			{
				Name:      "config",
				Usage:     "Create a configuration parameters file for this chart.",
				ArgsUsage: "CHART",
			},
			{
				Name:      "show",
				Aliases:   []string{"info"},
				Usage:     "Provide details about this package.",
				ArgsUsage: "CHART",
			},
			{
				Name: "scaffold",
			},
			{
				Name:      "list",
				Usage:     "list all deployed charts, optionally constraining by pattern.",
				ArgsUsage: "[PATTERN]",
			},
			{
				Name:      "deployments",
				Usage:     "given a chart, show all the deployments that reference it.",
				ArgsUsage: "CHART",
			},
		},
	}
}
