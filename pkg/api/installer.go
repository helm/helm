package api

import (
	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

type install struct {
	*action.Install
}

type Installer interface {
	Run(*chart.Chart, map[string]interface{}) (*release.Release, error)
	SetConfig(InstallConfig)
}

func (i *install) SetConfig(cfg InstallConfig) {
	i.ReleaseName = cfg.Name
	i.Namespace = cfg.Namespace
}

func NewInstall(ai *action.Install) *install {
	return &install{ai}
}

type MockInstall struct{ mock.Mock }

func (m *MockInstall) SetConfig(cfg InstallConfig) {
	m.Called(cfg)
}

func (m *MockInstall) Run(c *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(c, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}
