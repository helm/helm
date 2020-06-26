package api_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	"helm.sh/helm/v3/pkg/api"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type InstallerTestSuite struct {
	suite.Suite
	recorder        *httptest.ResponseRecorder
	server          *httptest.Server
	mockInstaller   *mockInstaller
	mockChartLoader *mockChartLoader
	appConfig       *cli.EnvSettings
}

func (s *InstallerTestSuite) SetupSuite() {
	logger.Setup("default")
}

func (s *InstallerTestSuite) SetupTest() {
	s.recorder = httptest.NewRecorder()
	s.mockInstaller = new(mockInstaller)
	s.mockChartLoader = new(mockChartLoader)
	s.appConfig = &cli.EnvSettings{
		RepositoryConfig: "./testdata/helm",
		PluginsDirectory: "./testdata/helm/plugin",
	}
	service := api.NewService(s.appConfig, s.mockChartLoader, s.mockInstaller, nil, nil)
	handler := api.Install(service)
	s.server = httptest.NewServer(handler)
}

func (s *InstallerTestSuite) TestShouldReturnDeployedStatusOnSuccessfulInstall() {
	chartName := "stable/redis-ha"
	body := fmt.Sprintf(`{
    "chart":"%s",
    "name": "redis-v5",
    "namespace": "something"}`, chartName)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/install", s.server.URL), strings.NewReader(body))
	s.mockChartLoader.On("LocateChart", chartName, s.appConfig).Return("./testdata/albatross", nil)
	icfg := api.ReleaseConfig{ChartName: chartName, Name: "redis-v5", Namespace: "something"}
	s.mockInstaller.On("SetConfig", icfg)
	release := &release.Release{Info: &release.Info{Status: release.StatusDeployed}}
	var vals map[string]interface{}
	//TODO: pass chart object and verify values present testdata chart yml
	s.mockInstaller.On("Run", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)

	resp, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	expectedResponse := `{"status":"deployed"}` + "\n"
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(s.T(), expectedResponse, string(respBody))
	require.NoError(s.T(), err)
	s.mockInstaller.AssertExpectations(s.T())
	s.mockChartLoader.AssertExpectations(s.T())
}

func (s *InstallerTestSuite) TestShouldReturnInternalServerErrorOnFailure() {
	chartName := "stable/redis-ha"
	body := fmt.Sprintf(`{
    "chart":"%s",
    "name": "redis-v5",
    "namespace": "something"}`, chartName)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/install", s.server.URL), strings.NewReader(body))
	s.mockChartLoader.On("LocateChart", chartName, s.appConfig).Return("./testdata/albatross", errors.New("Invalid chart"))

	resp, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
	expectedResponse := `{"error":"error in locating chart: Invalid chart"}` + "\n"
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(s.T(), expectedResponse, string(respBody))
	require.NoError(s.T(), err)
	s.mockInstaller.AssertExpectations(s.T())
	s.mockChartLoader.AssertExpectations(s.T())
}

func (s *InstallerTestSuite) TearDownTest() {
	s.server.Close()
}

func TestInstallAPI(t *testing.T) {
	suite.Run(t, new(InstallerTestSuite))
}

type mockInstaller struct{ mock.Mock }

func (m *mockInstaller) SetConfig(cfg api.ReleaseConfig) {
	m.Called(cfg)
}

func (m *mockInstaller) Run(c *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(c, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

type mockChartLoader struct{ mock.Mock }

func (m *mockChartLoader) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	args := m.Called(name, settings)
	return args.String(0), args.Error(1)
}
