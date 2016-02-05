package main

import (
	"errors"
	"os"

	"github.com/codegangsta/cli"
	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/format"
	"github.com/kubernetes/deployment-manager/chart"
)

func deploy(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		format.Err("First argument, filename, is required. Try 'helm deploy --help'")
		os.Exit(1)
	}

	props, err := parseProperties(c.String("properties"))
	if err != nil {
		format.Err("Failed to parse properties: %s", err)
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

	return doDeploy(d, c.GlobalString("host"), c.Bool("dry-run"))
}

func doDeploy(cfg *dep.Deployment, host string, dry bool) error {
	if cfg.Filename == "" {
		return errors.New("A filename must be specified. For a tar archive, this is the name of the root template in the archive.")
	}

	fi, err := os.Stat(cfg.Filename)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		format.Info("Chart is directory")
		c, err := chart.LoadDir(cfg.Filename)
		if err != nil {
			return err
		}

		//tdir, err := ioutil.TempDir("", "helm-")
		//if err != nil {
		//format.Warn("Could not create temporary directory. Using .")
		//tdir = "."
		//} else {
		//defer os.RemoveAll(tdir)
		//}
		tdir := "."
		tfile, err := chart.Save(c, tdir)
		if err != nil {
			return err
		}
		cfg.Filename = tfile

	}

	if !dry {
		if err := uploadTar(cfg.Filename); err != nil {
			return err
		}
	}

	return nil
}

func uploadTar(filename string) error {
	return nil
}
