package api

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

type install struct {
	*action.Install
}

type Installer interface {
	Run(*chart.Chart, map[string]interface{}) (*release.Release, error)
	SetConfig(ReleaseConfig)
}

func (i *install) SetConfig(cfg ReleaseConfig) {
	i.ReleaseName = cfg.Name
	i.Namespace = cfg.Namespace
}

func NewInstall(ai *action.Install) *install {
	return &install{ai}
}
