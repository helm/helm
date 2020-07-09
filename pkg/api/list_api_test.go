package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/api"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type ListTestSuite struct {
	suite.Suite
	recorder  *httptest.ResponseRecorder
	server    *httptest.Server
	mockList  *mockList
	appConfig *cli.EnvSettings
}

type mockList struct{ mock.Mock }

func (m *mockList) Run() ([]*release.Release, error) {
	args := m.Called()
	return args.Get(0).([]*release.Release), args.Error(1)
}

func (m *mockList) SetStateMask() {
	m.Called()
}

func (m *mockList) SetConfig(state action.ListStates, allNameSpaces bool) {
	m.Called(state, allNameSpaces)
}

func (s *ListTestSuite) SetupSuite() {
	logger.Setup("default")
}

func (s *ListTestSuite) SetupTest() {
	s.recorder = httptest.NewRecorder()
	s.mockList = new(mockList)
	s.appConfig = &cli.EnvSettings{
		RepositoryConfig: "./testdata/helm",
		PluginsDirectory: "./testdata/helm/plugin",
	}
	service := api.NewService(s.appConfig, nil, s.mockList, nil, nil, nil)
	handler := api.List(service)
	s.server = httptest.NewServer(handler)
}

func (s *ListTestSuite) TestShouldReturnReleasesWhenSuccessfulAPICall() {
	layout := "2006-01-02T15:04:05.000Z"
	str := "2014-11-12T11:45:26.371Z"
	timeFromStr, _ := time.Parse(layout, str)
	body := `{"release_status":"deployed"}`
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/list", s.server.URL), strings.NewReader(body))

	releases := []*release.Release{{Name: "test-release",
		Namespace: "test-namespace",
		Info:      &release.Info{Status: release.StatusDeployed, LastDeployed: timeFromStr},
		Version:   1,
		Chart:     &chart.Chart{Metadata: &chart.Metadata{Name: "test-release", Version: "0.1", AppVersion: "0.1"}},
	}}

	s.mockList.On("SetStateMask")
	s.mockList.On("SetConfig", action.ListDeployed, true)
	s.mockList.On("Run").Return(releases, nil)

	res, err := http.DefaultClient.Do(req)
	assert.Equal(s.T(), 200, res.StatusCode)

	var actualResponse api.ListResponse
	err = json.NewDecoder(res.Body).Decode(&actualResponse)

	expectedResponse := api.ListResponse{Error: "",
		Releases: []api.Release{{"test-release",
			"test-namespace",
			1,
			timeFromStr,
			release.StatusDeployed,
			"test-release-0.1",
			"0.1",
		}}}

	assert.Equal(s.T(), expectedResponse.Releases[0], actualResponse.Releases[0])
	require.NoError(s.T(), err)
	s.mockList.AssertExpectations(s.T())
}

func (s *ListTestSuite) TestShouldReturnBadRequestErrorIfItHasInvalidCharacter() {
	body := `{"release_status":"unknown""""}`
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/list", s.server.URL), strings.NewReader(body))

	res, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), 400, res.StatusCode)

	expectedResponse := "invalid character '\"' after object key:value pair"

	var actualResponse api.ListResponse
	err = json.NewDecoder(res.Body).Decode(&actualResponse)

	assert.Equal(s.T(), expectedResponse, actualResponse.Error)
	require.NoError(s.T(), err)
}

func (s *ListTestSuite) TearDownTest() {
	s.server.Close()
}

func TestListAPI(t *testing.T) {
	suite.Run(t, new(ListTestSuite))
}
