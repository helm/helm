package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/format"
)

var version = "0.0.1"

func main() {
	app := cli.NewApp()
	app.Name = "helm"
	app.Version = version
	app.Usage = `Deploy and manage packages.`
	app.Commands = commands()

	app.Run(os.Args)
}

func commands() []cli.Command {
	return []cli.Command{
		{
			Name:        "install",
			Usage:       "Initialize the client and install DM on Kubernetes.",
			Description: ``,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "dry-run",
					Usage: "Show what would be installed, but don't install anything.",
				},
			},
			Action: func(c *cli.Context) {
				if err := install(c.Bool("dry-run")); err != nil {
					format.Error(err.Error())
					os.Exit(1)
				}
			},
		},
		{
			Name:      "target",
			Usage:     "Displays information about cluster.",
			ArgsUsage: "",
			Action: func(c *cli.Context) {
				if err := target(c.Bool("dry-run")); err != nil {
					format.Error(err.Error())
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "dry-run",
					Usage: "Only display the underlying kubectl commands.",
				},
			},
		},
		{
			Name: "doctor",
		},
		{
			Name: "deploy",
		},
		{
			Name: "search",
		},
	}
}
