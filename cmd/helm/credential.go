package main

import (
	"github.com/codegangsta/cli"
)

func init() {
	addCommands(credCommands())
}

func credCommands() cli.Command {
	return cli.Command{
		Name:    "credential",
		Aliases: []string{"cred"},
		Usage:   "Perform repository credential operations.",
		Subcommands: []cli.Command{
			{
				Name:  "add",
				Usage: "Add a credential to the remote manager.",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "file,f",
						Usage: "A JSON file with credential information.",
					},
				},
				ArgsUsage: "CREDENTIAL",
			},
			{
				Name:      "list",
				Usage:     "List the credentials on the remote manager.",
				ArgsUsage: "",
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     "Remove a credential from the remote manager.",
				ArgsUsage: "CREDENTIAL",
			},
		},
	}
}
