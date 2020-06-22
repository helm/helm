package api

import (
	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/cli"
)

type chartloader interface {
	LocateChart(name string, settings *cli.EnvSettings) (string, error)
}

type MockChartLoader struct{ mock.Mock }

func (m *MockChartLoader) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	args := m.Called(name, settings)
	return args.String(0), args.Error(1)
}

