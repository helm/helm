package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(lintCmd())
}

func lintCmd() cli.Command {
	return cli.Command{
		Name:      "lint",
		Usage:     "Evaluate a chart's conformance to the specification.",
		ArgsUsage: "PATH [PATH...]",
	}
}
