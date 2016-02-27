package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/format"
	"github.com/kubernetes/deployment-manager/chart"
)

func init() {
	addCommands(packageCmd())
}

func packageCmd() cli.Command {
	return cli.Command{
		Name:      "package",
		Aliases:   []string{"pack"},
		Usage:     "Given a chart directory, package it into a release.",
		ArgsUsage: "PATH",
		Action:    func(c *cli.Context) { run(c, pack) },
	}
}

func pack(cxt *cli.Context) error {
	args := cxt.Args()
	if len(args) < 1 {
		return errors.New("'helm package' requires a path to a chart directory as an argument")
	}

	dir := args[0]
	if fi, err := os.Stat(dir); err != nil {
		return fmt.Errorf("Could not find directory %s: %s", dir, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("Not a directory: %s", dir)
	}

	c, err := chart.LoadDir(dir)
	if err != nil {
		return fmt.Errorf("Failed to load %s: %s", dir, err)
	}

	fname, err := chart.Save(c, ".")
	format.Msg(fname)
	return nil
}
