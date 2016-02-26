package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(statusCommand())
}

func statusCommand() cli.Command {
	return cli.Command{
		Name:      "status",
		Usage:     "Provide status on a named deployment.",
		ArgsUsage: "DEPLOYMENT",
	}
}
