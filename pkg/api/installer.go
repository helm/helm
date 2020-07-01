package api

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

type Installer struct {
	*action.Install
}

type InstallRunner interface {
	Run(*chart.Chart, map[string]interface{}) (*release.Release, error)
	SetConfig(ReleaseConfig)
}

func (i *Installer) SetConfig(cfg ReleaseConfig) {
	i.ReleaseName = cfg.Name
	i.Namespace = cfg.Namespace
}

func NewInstall(ai *action.Install) *Installer {
	return &Installer{ai}
}
