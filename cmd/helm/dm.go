package main

import (
	"errors"

	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

// ErrAlreadyInstalled indicates that DM is already installed.
var ErrAlreadyInstalled = errors.New("Already Installed")

func install(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Install(runner)
	if err != nil {
		format.Err("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func uninstall(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Uninstall(runner)
	if err != nil {
		format.Err("Error uninstalling: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func getKubectlRunner(dryRun bool) kubectl.Runner {
	if dryRun {
		return &kubectl.PrintRunner{}
	}
	return &kubectl.RealRunner{}
}
