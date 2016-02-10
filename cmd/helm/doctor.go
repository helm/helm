package main

import (
	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

func init() {
	addCommands(doctorCmd())
}

func doctorCmd() cli.Command {
	return cli.Command{
		Name:      "doctor",
		Usage:     "Run a series of checks for necessary prerequisites.",
		ArgsUsage: "",
		Action:    func(c *cli.Context) { run(c, doctor) },
	}
}

func doctor(c *cli.Context) error {
	var runner kubectl.Runner
	runner = &kubectl.RealRunner{}
	if dm.IsInstalled(runner) {
		format.Success("You have everything you need. Go forth my friend!")
	} else {
		format.Warning("Looks like you don't have DM installed.\nRun: `helm install`")
	}

	return nil
}
