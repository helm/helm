package main

import (
	"errors"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/format"
)

func init() {
	addCommands(deleteCmd())
}

func deleteCmd() cli.Command {
	return cli.Command{
		Name:      "delete",
		Usage:     "Deletes the supplied deployment",
		ArgsUsage: "DEPLOYMENT",
		Action:    func(c *cli.Context) { run(c, deleteDeployment) },
	}
}

func deleteDeployment(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.New("First argument, deployment name, is required. Try 'helm get --help'")
	}
	name := args[0]
	deployment, err := client(c).DeleteDeployment(name)
	if err != nil {
		return err
	}
	return format.YAML(deployment)
}
