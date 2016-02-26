package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(repoCommands())
}

func repoCommands() cli.Command {
	return cli.Command{
		Name:    "repository",
		Aliases: []string{"repo"},
		Usage:   "Perform repository operations.",
		Subcommands: []cli.Command{
			{
				Name:      "add",
				Usage:     "Add a repository to the remote manager.",
				ArgsUsage: "REPOSITORY",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "cred",
						Usage: "The name of the credential.",
					},
				},
			},
			{
				Name:      "show",
				Usage:     "Show the repository details for a given repository.",
				ArgsUsage: "REPOSITORY",
			},
			{
				Name:      "list",
				Usage:     "List the repositories on the remote manager.",
				ArgsUsage: "",
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Remove a repository from the remote manager.",
				ArgsUsage: "REPOSITORY",
			},
		},
	}
}
