package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx         context.Context
	installer   *mockInstaller
	upgrader    *mockUpgrader
	history     *mockHistory
	chartloader *mockChartLoader
	svc         Service
	settings    *cli.EnvSettings
}

func (s *ServiceTestSuite) SetupTest() {
	logger.Setup("")
	s.ctx = context.Background()
	s.installer = new(mockInstaller)
	s.upgrader = new(mockUpgrader)
	s.history = new(mockHistory)
	s.chartloader = new(mockChartLoader)
	s.settings = &cli.EnvSettings{}
	s.svc = NewService(s.settings, s.chartloader, s.installer, s.upgrader, s.history)
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnInvalidChart() {
	chartName := "stable/invalid-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals chartValues
	s.chartloader.On("LocateChart", chartName, s.settings).Return("", errors.New("Unable to find chart"))

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error in locating chart: Unable to find chart")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertNotCalled(t, "SetConfig")
	s.installer.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnLocalChartReference() {
	chartName := "./some/local-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals chartValues

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error request validation: cannot refer local chart")
	s.chartloader.AssertNotCalled(t, "LocateChart")
	s.installer.AssertNotCalled(t, "SetConfig")
	s.installer.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnFailedInstallRun() {
	chartName := "stable/valid-chart"
	cfg := ReleaseConfig{
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
	cfg := ReleaseConfig{
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
	assert.Equal(t, res.status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeInstallTrueShouldInstallChart() {
	chartName := "stable/valid-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	s.upgrader.On("GetInstall").Return(true)
	s.history.On("Run", "some-component").Return([]*release.Release{}, driver.ErrReleaseNotFound)

	s.installer.On("SetConfig", cfg)
	release := &release.Release{Name: "some-comp-release", Info: &release.Info{Status: release.StatusDeployed}}
	s.installer.On("Run", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)
	res, err := s.svc.Upgrade(s.ctx, cfg, vals)

	t := s.T()
	assert.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, res.status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeInstallFalseShouldNotInstallChart() {
	chartName := "stable/valid-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	s.upgrader.On("GetInstall").Return(false)

	s.upgrader.On("SetConfig", cfg)
	release := &release.Release{Name: "some-comp-release", Info: &release.Info{Status: release.StatusDeployed}}
	s.upgrader.On("Run", "some-component", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)
	res, err := s.svc.Upgrade(s.ctx, cfg, vals)

	t := s.T()
	assert.NoError(t, err)
	require.NotNil(t, res)
	s.installer.AssertNotCalled(t, "Run")
	s.history.AssertNotCalled(t, "Run")
	assert.Equal(t, res.status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeShouldReturnErrorOnFailedUpgradeRun() {
	chartName := "stable/valid-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	s.upgrader.On("GetInstall").Return(false)
	s.upgrader.On("SetConfig", cfg)
	release := &release.Release{Name: "some-comp-release", Info: &release.Info{Status: release.StatusDeployed}}
	s.upgrader.On("Run", "some-component", mock.AnythingOfType("*chart.Chart"), vals).Return(release, errors.New("cluster issue"))

	res, err := s.svc.Upgrade(s.ctx, cfg, vals)
	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error in upgrading chart: cluster issue")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeShouldReturnResultOnSuccess() {
	chartName := "stable/valid-chart"
	cfg := ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals map[string]interface{}
	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
	s.upgrader.On("GetInstall").Return(false)
	s.upgrader.On("SetConfig", cfg)
	release := &release.Release{Name: "some-comp-release", Info: &release.Info{Status: release.StatusDeployed}}
	s.upgrader.On("Run", "some-component", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)

	res, err := s.svc.Upgrade(s.ctx, cfg, vals)
	t := s.T()
	assert.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, res.status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.upgrader.AssertExpectations(t)
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

type mockChartLoader struct{ mock.Mock }

func (m *mockChartLoader) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	args := m.Called(name, settings)
	return args.String(0), args.Error(1)
}

type mockInstaller struct{ mock.Mock }

type mockUpgrader struct{ mock.Mock }

type mockHistory struct{ mock.Mock }

func (m *mockInstaller) SetConfig(cfg ReleaseConfig) {
	m.Called(cfg)
}

func (m *mockInstaller) Run(c *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(c, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *mockUpgrader) GetInstall() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

func (m *mockUpgrader) Run(name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(name, chart, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *mockUpgrader) SetConfig(cfg ReleaseConfig) {
	_ = m.Called(cfg)
}

func (m *mockHistory) Run(name string) ([]*release.Release, error) {
	args := m.Called(name)
	return args.Get(0).([]*release.Release), args.Error(1)
}
