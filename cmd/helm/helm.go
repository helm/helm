package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
)

var version = "0.0.1"

var commands []cli.Command

func init() {
	addCommands(cmds()...)
}

func main() {
	app := cli.NewApp()
	app.Name = "helm"
	app.Version = version
	app.Usage = `Deploy and manage packages.`
	app.Commands = commands

	// TODO: make better
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host,u",
			Usage:  "The URL of the DM server.",
			EnvVar: "HELM_HOST",
			Value:  "https://localhost:8181/FIXME_NOT_RIGHT",
		},
		cli.IntFlag{
			Name:  "timeout",
			Usage: "Time in seconds to wait for response",
			Value: 10,
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable verbose debugging output",
		},
	}
	app.Run(os.Args)
}

func cmds() []cli.Command {
	return []cli.Command{
		{
			Name:        "init",
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
					format.Err("%s (Run 'helm doctor' for more information)", err)
					os.Exit(1)
				}
			},
		},
		{
			Name: "search",
		},
	}
}

func addCommands(cmds ...cli.Command) {
	commands = append(commands, cmds...)
}

func run(c *cli.Context, f func(c *cli.Context) error) {
	if err := f(c); err != nil {
		os.Stderr.Write([]byte(err.Error()))
		os.Exit(1)
	}
}

func client(c *cli.Context) *dm.Client {
	host := c.GlobalString("host")
	debug := c.GlobalBool("debug")
	timeout := c.GlobalInt("timeout")
	return dm.NewClient(host).SetDebug(debug).SetTimeout(timeout)
}
