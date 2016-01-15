package main

import (
	"errors"

	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/format"
)

func deploy(cfg *dep.Deployment, dry bool) error {
	if dry {
		format.Error("Not implemented: --dry-run")
	}
	if cfg.Filename == "" {
		return errors.New("A filename must be specified. For a tar archive, this is the name of the root template in the archive.")
	}

	if err := cfg.Commit(); err != nil {
		format.Error("Failed to commit deployment: %s", err)
		return err
	}

	return nil
}
