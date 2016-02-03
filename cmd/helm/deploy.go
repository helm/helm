package main

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/codegangsta/cli"
	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/format"
)

func deploy(c *cli.Context) error {
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

	//return doDeploy(d, c.GlobalString("host"), c.Bool("dry-run"))
	return nil
}

func doDeploy(cfg *dep.Deployment, host string, dry bool) error {
	if cfg.Filename == "" {
		return errors.New("A filename must be specified. For a tar archive, this is the name of the root template in the archive.")
	}

	if err := cfg.Prepare(); err != nil {
		format.Error("Failed to prepare deployment: %s", err)
		return err
	}

	// For a dry run, print the template and exit.
	if dry {
		format.Info("Template prepared for %s", cfg.Template.Name)
		data, err := json.MarshalIndent(cfg.Template, "", "\t")
		if err != nil {
			return err
		}
		format.Msg(string(data))
		return nil
	}

	if err := cfg.Commit(host); err != nil {
		format.Error("Failed to commit deployment: %s", err)
		return err
	}

	return nil
}
