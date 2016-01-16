package main

import (
	"encoding/json"
	"errors"

	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/format"
)

func deploy(cfg *dep.Deployment, host string, dry bool) error {
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
