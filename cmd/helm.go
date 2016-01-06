package main

import (
	"os"

	"github.com/codegangsta/cli"
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
			Action:      func(c *cli.Context) { install() },
		},
		{
			Name: "target",
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
