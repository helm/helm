package main

import (
	"errors"

	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

var ErrAlreadyInstalled error = errors.New("Already Installed")

func install(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Install(runner)
	if err != nil {
		format.Error("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func uninstall(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Uninstall(runner)
	if err != nil {
		format.Error("Error uninstalling: %s %s", out, err)
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
