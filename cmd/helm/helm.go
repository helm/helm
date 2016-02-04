package main

import (
	"os"

	"github.com/codegangsta/cli"
	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/format"
)

var version = "0.0.1"

func main() {
	app := cli.NewApp()
	app.Name = "helm"
	app.Version = version
	app.Usage = `Deploy and manage packages.`
	app.Commands = commands()

	// TODO: make better
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host,u",
			Usage:  "The URL of the DM server.",
			EnvVar: "HELM_HOST",
			Value:  "https://localhost:8181/FIXME_NOT_RIGHT",
		},
	}

	app.Run(os.Args)
}

func commands() []cli.Command {
	return []cli.Command{
		{
			Name:  "dm",
			Usage: "Manage DM on Kubernetes",
			Subcommands: []cli.Command{
				{
					Name:        "install",
					Usage:       "Install DM on Kubernetes.",
					Description: ``,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "dry-run",
							Usage: "Show what would be installed, but don't install anything.",
						},
					},
					Action: func(c *cli.Context) {
						if err := install(c.Bool("dry-run")); err != nil {
							format.Error("%s (Run 'helm doctor' for more information)", err)
							os.Exit(1)
						}
					},
				},
				{
					Name:        "uninstall",
					Usage:       "Uninstall the DM from Kubernetes.",
					Description: ``,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "dry-run",
							Usage: "Show what would be installed, but don't install anything.",
						},
					},
					Action: func(c *cli.Context) {
						if err := uninstall(c.Bool("dry-run")); err != nil {
							format.Error("%s (Run 'helm doctor' for more information)", err)
							os.Exit(1)
						}
					},
				},
				{
					Name:  "status",
					Usage: "Show status of DM.",
					Action: func(c *cli.Context) {
						format.Error("Not yet implemented")
						os.Exit(1)
					},
				},
				{
					Name:      "target",
					Usage:     "Displays information about cluster.",
					ArgsUsage: "",
					Action: func(c *cli.Context) {
						if err := target(c.Bool("dry-run")); err != nil {
							format.Error("%s (Is the cluster running?)", err)
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
			},
		},
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
					format.Error("%s (Run 'helm doctor' for more information)", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:      "doctor",
			Usage:     "Run a series of checks for necessary prerequisites.",
			ArgsUsage: "",
			Action: func(c *cli.Context) {
				if err := doctor(); err != nil {
					format.Error("%s", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:    "deploy",
			Aliases: []string{"install"},
			Usage:   "Deploy a chart into the cluster.",
			Action: func(c *cli.Context) {

				args := c.Args()
				if len(args) < 1 {
					format.Error("First argument, filename, is required. Try 'helm deploy --help'")
					os.Exit(1)
				}

				props, err := parseProperties(c.String("properties"))
				if err != nil {
					format.Error("Failed to parse properties: %s", err)
					os.Exit(1)
				}

				d := &dep.Deployment{
					Name:       c.String("Name"),
					Properties: props,
					Filename:   args[0],
					Imports:    args[1:],
					Repository: c.String("repository"),
				}

				if c.Bool("stdin") {
					d.Input = os.Stdin
				}

				if err := deploy(d, c.GlobalString("host"), c.Bool("dry-run")); err != nil {
					format.Error("%s (Try running 'helm doctor')", err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "dry-run",
					Usage: "Only display the underlying kubectl commands.",
				},
				cli.BoolFlag{
					Name:  "stdin,i",
					Usage: "Read a configuration from STDIN.",
				},
				cli.StringFlag{
					Name:  "name",
					Usage: "Name of deployment, used for deploy and update commands (defaults to template name)",
				},
				// TODO: I think there is a Generic flag type that we can implement parsing with.
				cli.StringFlag{
					Name:  "properties,p",
					Usage: "A comma-separated list of key=value pairs: 'foo=bar,foo2=baz'.",
				},
				cli.StringFlag{
					// FIXME: This is not right. It's sort of a half-baked forward
					// port of dm.go.
					Name:  "repository",
					Usage: "The default repository",
					Value: "kubernetes/application-dm-templates",
				},
			},
		},
		{
			Name: "search",
		},
		listCmd(),
	}
}
