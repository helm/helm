package main

import (
	"errors"

	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

var ErrAlreadyInstalled error = errors.New("Already Installed")

func install(dryRun bool) error {
	var runner kubectl.Runner
	if dryRun {
		runner = &kubectl.PrintRunner{}
	} else {
		runner = &kubectl.RealRunner{}
		if dm.IsInstalled(runner) {
			return ErrAlreadyInstalled
		}
	}
	out, err := dm.Install(runner)
	if err != nil {
		format.Error("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}
