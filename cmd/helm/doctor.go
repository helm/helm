package main

import (
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

func doctor() error {
	var runner kubectl.Runner
	runner = &kubectl.RealRunner{}
	if dm.IsInstalled(runner) {
		format.Success("You have everything you need. Go forth my friend!")
	} else {
		format.Warning("Looks like you don't have DM installed.\nRun: `helm install`")
	}

	return nil
}
