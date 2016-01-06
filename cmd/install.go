package main

import (
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

func install() error {
	runner := &kubectl.PrintRunner{}
	out, err := dm.Install(runner)
	if err != nil {
		format.Error("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}
