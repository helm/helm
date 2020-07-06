package api_test

import (
	"context"
	"errors"
	"testing"

	"helm.sh/helm/v3/pkg/time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/api"

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
	upgrader    *mockUpgrader
	history     *mockHistory
	installer   *mockInstall
	chartloader *mockChartLoader
	lister      *mockList
	svc         api.Service
	settings    *cli.EnvSettings
}

func (s *ServiceTestSuite) SetupTest() {
	logger.Setup("")
	s.settings = &cli.EnvSettings{}
	s.chartloader = new(mockChartLoader)
	s.lister = new(mockList)
	s.installer = new(mockInstall)
	s.upgrader = new(mockUpgrader)
	s.history = new(mockHistory)
	s.ctx = context.Background()
	s.svc = api.NewService(s.settings, s.chartloader, s.lister, s.installer, s.upgrader, s.history)
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnInvalidChart() {
	var vals api.ChartValues
	chartName := "stable/invalid-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

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
	var vals api.ChartValues
	chartName := "./some/local-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

	res, err := s.svc.Install(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error request validation: cannot refer local chart")
	s.chartloader.AssertNotCalled(t, "LocateChart")
	s.installer.AssertNotCalled(t, "SetConfig")
	s.installer.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestInstallShouldReturnErrorOnFailedInstallRun() {
	var release *release.Release
	var vals map[string]interface{}
	chartName := "stable/valid-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

	s.chartloader.On("LocateChart", chartName, s.settings).Return("testdata/albatross", nil)
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
	var vals map[string]interface{}
	chartName := "stable/valid-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

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

func (s *ServiceTestSuite) TestUpgradeInstallTrueShouldInstallChart() {
	var vals map[string]interface{}
	chartName := "stable/valid-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

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
	assert.Equal(t, res.Status, "deployed")
	s.upgrader.AssertNotCalled(t, "Run")
	s.chartloader.AssertExpectations(t)
	s.upgrader.AssertExpectations(t)
	s.history.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeInstallFalseShouldNotInstallChart() {
	chartName := "stable/valid-chart"
	cfg := api.ReleaseConfig{
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
	assert.Equal(t, res.Status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.installer.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeShouldReturnErrorOnFailedUpgradeRun() {
	chartName := "stable/valid-chart"
	cfg := api.ReleaseConfig{
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
	cfg := api.ReleaseConfig{
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
	assert.Equal(t, res.Status, "deployed")
	s.chartloader.AssertExpectations(t)
	s.upgrader.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestUpgradeValidateFailShouldResultFailure() {
	var vals api.ChartValues
	chartName := "./some/local-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}

	res, err := s.svc.Upgrade(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error request validation: cannot refer local chart")
	s.chartloader.AssertNotCalled(t, "LocateChart")
	s.upgrader.AssertNotCalled(t, "SetConfig")
	s.upgrader.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestUpgradeShouldReturnErrorOnInvalidChart() {
	chartName := "stable/invalid-chart"
	cfg := api.ReleaseConfig{
		Name:      "some-component",
		Namespace: "hermes",
		ChartName: chartName,
	}
	var vals api.ChartValues
	s.chartloader.On("LocateChart", chartName, s.settings).Return("", errors.New("Unable to find chart"))

	res, err := s.svc.Upgrade(s.ctx, cfg, vals)

	t := s.T()
	assert.Nil(t, res)
	assert.EqualError(t, err, "error in locating chart: Unable to find chart")
	s.chartloader.AssertExpectations(t)
	s.upgrader.AssertNotCalled(t, "SetConfig")
	s.upgrader.AssertNotCalled(t, "Run")
}

func (s *ServiceTestSuite) TestListShouldReturnErrorOnFailureOfListRun() {
	var releases []*release.Release
	releaseStatus := "deployed"
	s.lister.On("SetState", action.ListDeployed)
	s.lister.On("SetStateMask")
	s.lister.On("Run").Return(releases, errors.New("cluster issue"))

	res, err := s.svc.List(releaseStatus)

	t := s.T()
	assert.Error(t, err, "cluster issue")
	assert.Nil(t, res)
	s.lister.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestListShouldReturnAllReleasesIfNoFilterIsPassed() {
	layout := "2006-01-02T15:04:05.000Z"
	str := "2014-11-12T11:45:26.371Z"
	releaseStatus := ""
	var releases []*release.Release
	timeFromStr, _ := time.Parse(layout, str)
	releases = append(releases,
		&release.Release{Name: "test-release",
			Namespace: "test-namespace",
			Info:      &release.Info{Status: release.StatusDeployed, LastDeployed: timeFromStr},
			Version:   1,
			Chart:     &chart.Chart{Metadata: &chart.Metadata{Name: "test-release", Version: "0.1", AppVersion: "0.1"}},
		})
	s.lister.On("SetState", action.ListAll)
	s.lister.On("SetStateMask")

	s.lister.On("Run").Return(releases, nil)

	res, err := s.svc.List(releaseStatus)

	t := s.T()
	assert.NoError(t, err)
	require.NotNil(t, res)

	var response []api.Release
	response = append(response, api.Release{Name: "test-release",
		Namespace:  "test-namespace",
		Revision:   1,
		Updated:    timeFromStr,
		Status:     release.StatusDeployed,
		Chart:      "test-release-0.1",
		AppVersion: "0.1",
	})

	assert.Equal(t, 1, len(res))
	assert.Equal(t, "test-release", response[0].Name)
	assert.Equal(t, "test-namespace", response[0].Namespace)
	assert.Equal(t, 1, response[0].Revision)
	assert.Equal(t, timeFromStr, response[0].Updated)
	assert.Equal(t, release.StatusDeployed, response[0].Status)
	assert.Equal(t, "test-release-0.1", response[0].Chart)
	assert.Equal(t, "0.1", response[0].AppVersion)
	assert.Equal(t, response, releases[0])
	s.lister.AssertExpectations(t)
}

func (s *ServiceTestSuite) TestListShouldReturnErrorIfInvalidStatusIsPassedAsFilter() {
	releaseStatus := "invalid"
	_, err := s.svc.List(releaseStatus)

	t := s.T()
	assert.EqualError(t, err, "invalid release status")
}

func (s *ServiceTestSuite) TestListShouldReturnDeployedReleasesIfDeployedIsPassedAsFilter() {
	var releases []*release.Release
	releaseStatus := "deployed"
	s.lister.On("SetState", action.ListDeployed)
	s.lister.On("SetStateMask")
	s.lister.On("Run").Return(releases, nil)

	_, err := s.svc.List(releaseStatus)

	t := s.T()
	assert.NoError(t, err)
	s.lister.AssertExpectations(t)
}

type mockInstall struct{ mock.Mock }

func (m *mockInstall) SetConfig(cfg api.ReleaseConfig) {
	m.Called(cfg)
}

func (m *mockInstall) Run(c *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(c, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

type mockChartLoader struct{ mock.Mock }

func (m *mockChartLoader) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	args := m.Called(name, settings)
	return args.String(0), args.Error(1)
}

type mockUpgrader struct{ mock.Mock }

func (m *mockUpgrader) Run(name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(name, chart, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *mockUpgrader) SetConfig(cfg api.ReleaseConfig) {
	_ = m.Called(cfg)
}

func (m *mockUpgrader) GetInstall() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

type mockHistory struct{ mock.Mock }

func (m *mockHistory) Run(name string) ([]*release.Release, error) {
	args := m.Called(name)
	return args.Get(0).([]*release.Release), args.Error(1)
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}
