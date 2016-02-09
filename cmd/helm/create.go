package main

import (
	"errors"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/deployment-manager/chart"
)

func init() {
	addCommands(createCmd())
}

func createCmd() cli.Command {
	return cli.Command{
		Name:   "create",
		Usage:  "Create a new local chart for editing.",
		Action: func(c *cli.Context) { run(c, create) },
	}
}

func create(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("'helm create' requires a chart name as an argument")
	}

	cf := &chart.Chartfile{
		Name:        args[0],
		Description: "Created by Helm",
		Version:     "0.1.0",
	}

	_, err := chart.Create(cf, ".")
	return err
}
