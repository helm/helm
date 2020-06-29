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

type UpgradeTestSuite struct {
	suite.Suite
	recorder        *httptest.ResponseRecorder
	server          *httptest.Server
	mockUpgrader    *mockUpgrader
	mockHistory     *mockHistory
	mockChartLoader *mockChartLoader
	appConfig       *cli.EnvSettings
}

func (s *UpgradeTestSuite) SetupSuite() {
	logger.Setup("default")
}

func (s *UpgradeTestSuite) SetupTest() {
	s.recorder = httptest.NewRecorder()
	s.mockUpgrader = new(mockUpgrader)
	s.mockHistory = new(mockHistory)
	s.mockChartLoader = new(mockChartLoader)
	s.appConfig = &cli.EnvSettings{
		RepositoryConfig: "./testdata/helm",
		PluginsDirectory: "./testdata/helm/plugin",
	}
	service := api.NewService(s.appConfig, s.mockChartLoader, nil, s.mockUpgrader, s.mockHistory)
	handler := api.Upgrade(service)
	s.server = httptest.NewServer(handler)
}

func (s *UpgradeTestSuite) TestShouldReturnDeployedStatusOnSuccessfulUpgrade() {
	chartName := "stable/redis-ha"
	body := fmt.Sprintf(`{
    "chart":"%s",
    "name": "redis-v5",
    "namespace": "something"}`, chartName)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/upgrade", s.server.URL), strings.NewReader(body))
	s.mockChartLoader.On("LocateChart", chartName, s.appConfig).Return("./testdata/albatross", nil)
	ucfg := api.ReleaseConfig{ChartName: chartName, Name: "redis-v5", Namespace: "something"}
	s.mockUpgrader.On("GetInstall").Return(false)
	s.mockUpgrader.On("SetConfig", ucfg)
	release := &release.Release{Info: &release.Info{Status: release.StatusDeployed}}
	var vals map[string]interface{}
	//TODO: pass chart object and verify values present testdata chart yml
	s.mockUpgrader.On("Run", "redis-v5", mock.AnythingOfType("*chart.Chart"), vals).Return(release, nil)

	resp, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	expectedResponse := `{"status":"deployed"}` + "\n"
	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(expectedResponse)
	fmt.Println(respBody)

	assert.Equal(s.T(), expectedResponse, string(respBody))
	require.NoError(s.T(), err)
	s.mockUpgrader.AssertExpectations(s.T())
	s.mockChartLoader.AssertExpectations(s.T())
}

func (s *UpgradeTestSuite) TestShouldReturnInternalServerErrorOnFailure() {
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
	s.mockUpgrader.AssertExpectations(s.T())
	s.mockChartLoader.AssertExpectations(s.T())
}

func (s *UpgradeTestSuite) TearDownTest() {
	s.server.Close()
}

func TestUpgradeAPI(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

type mockUpgrader struct{ mock.Mock }

type mockHistory struct{ mock.Mock }

func (m *mockUpgrader) Run(name string, chart *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	args := m.Called(name, chart, vals)
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *mockUpgrader) GetInstall() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

func (m *mockUpgrader) SetConfig(cfg api.ReleaseConfig) {
	_ = m.Called(cfg)
}

func (m *mockHistory) Run(name string) ([]*release.Release, error) {
	args := m.Called(name)
	return args.Get(0).([]*release.Release), args.Error(1)
}
