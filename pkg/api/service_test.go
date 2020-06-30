package api_test

import (
	"context"
	"errors"
	"helm.sh/helm/v3/pkg/api"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx         context.Context
	installer   *mockInstall
	chartloader *mockChartLoader
	lister		*mockList
	svc         api.Service
	settings    *cli.EnvSettings
}

func (s *ServiceTestSuite) SetupTest() {
	logger.Setup("")
	s.ctx = context.Background()
	s.installer = new(mockInstall)
	s.lister = new(mockList)
	s.chartloader = new(mockChartLoader)
	s.settings = &cli.EnvSettings{}
	s.svc = api.NewService(s.settings, s.chartloader, s.installer, s.lister)
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnInvalidChart() {
	chartName := "stable/invalid-chart"
	cfg := api.InstallConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals api.ChartValues
	s.chartloader.On("LocateChart", chartName, s.settings).Return("", errors.New("Unable to find chart"))

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error in locating chart: Unable to find chart")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertNotCalled(t, "SetConfig")
	s.installer.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnFailedIntallRun() {
	chartName := "stable/valid-chart"
	cfg := api.InstallConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	var release *release.Release
	s.installer.On("SetConfig", cfg)
	s.installer.On("Run", mock.AnythingOfType("*chart.Chart"), vals).Return(release, errors.New("cluster issue"))

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error in installing chart: cluster issue")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestInstallShouldReturnResultOnSuccess() {
	chartName := "stable/valid-chart"
	cfg := api.InstallConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	s.installer.On("SetConfig", cfg)
	release := &release.Release{Name: "some-comp-release", Info: &release.Info{Status: release.StatusDeployed}}
	s.installer.On("Run", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, res.Status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}