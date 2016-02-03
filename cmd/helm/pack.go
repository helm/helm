package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/format"
	"github.com/kubernetes/deployment-manager/chart"
)

func pack(cxt *cli.Context) error {
	args := cxt.Args()
	if len(args) < 1 {
		return fmt.Errorf("'helm package' requires a path to a chart directory as an argument.")
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
