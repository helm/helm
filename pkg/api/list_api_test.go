package api_test

import (
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

func (m *mockList) SetState(state action.ListStates) {
	m.Called(state)
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
	body := `{"release_status":"deployed"}`
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/list", s.server.URL), strings.NewReader(body))

	var releases []*release.Release
	releases = append(releases,
		&release.Release{Name: "test-release",
			Namespace: "test-namespace",
			Info:      &release.Info{Status: release.StatusDeployed}})

	s.mockList.On("SetStateMask")
	s.mockList.On("SetState", action.ListDeployed)
	s.mockList.On("Run").Return(releases, nil)

	resp, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), 200, resp.StatusCode)

	expectedResponse := `{"Releases":[{"release":"test-release","namespace":"test-namespace"}]}`
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(s.T(), expectedResponse, string(respBody))

	require.NoError(s.T(), err)
	s.mockList.AssertExpectations(s.T())
}

func (s *ListTestSuite) TestShouldReturnBadRequestErrorIfItHasInvalidCharacter() {
	body := `{"request_id":"test-request-id""""}`
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/list", s.server.URL), strings.NewReader(body))

	resp, err := http.DefaultClient.Do(req)

	assert.Equal(s.T(), 400, resp.StatusCode)

	expectedResponse := `{"error":"invalid character '\"' after object key:value pair","Releases":null}`
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(s.T(), expectedResponse, string(respBody))
	require.NoError(s.T(), err)
}

func (s *ListTestSuite) TearDownTest() {
	s.server.Close()
}

func TestListAPI(t *testing.T) {
	suite.Run(t, new(ListTestSuite))
}
