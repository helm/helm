/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type RegistryScopeTestSuite struct {
	TestRegistry
}

// authRequest captures the fields of a token request that the test auth server
// receives. The oras-go v3 auth client may issue the request either as a GET
// (query params) or as an OAuth2 POST (form body); reading the parsed form
// covers both.
type authRequest struct {
	path    string
	service string
	scope   string
}

func captureAuthRequest(requests chan<- authRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		requests <- authRequest{
			path:    r.URL.Path,
			service: r.Form.Get("service"),
			scope:   r.Form.Get("scope"),
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (suite *RegistryScopeTestSuite) SetupSuite() {
	// set registry use token auth
	setup(&suite.TestRegistry, true, true, "token")
}

func (suite *RegistryScopeTestSuite) TearDownSuite() {
	teardown(&suite.TestRegistry)
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *RegistryScopeTestSuite) Test_1_Check_Push_Request_Scope() {
	requests := make(chan authRequest, 1)
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.NoError(err, "no error creating server listener")

	ts := httptest.NewUnstartedServer(captureAuthRequest(requests))
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	// basic push, good ref
	testingChartCreationTime := "1977-09-02T22:04:05Z"
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.NoError(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.NoError(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptCreationTime(testingChartCreationTime))
	suite.Error(err, "error pushing good ref because auth server doesn't give proper token")

	// check the request that the authentication server received
	select {
	case req := <-requests:
		suite.Equal("/auth", req.path)
		suite.Equal("testservice", req.service)
		suite.Contains(req.scope, "repository:testrepo/local-subchart:pull,push")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("timeout waiting for auth request")
	}
}

func (suite *RegistryScopeTestSuite) Test_2_Check_Pull_Request_Scope() {
	requests := make(chan authRequest, 1)
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.NoError(err, "no error creating server listener")

	ts := httptest.NewUnstartedServer(captureAuthRequest(requests))
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	// Simple pull, chart only
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.NoError(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.NoError(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Pull(ref)
	suite.Error(err, "error pulling a simple chart because auth server doesn't give proper token")

	// check the request that the authentication server received
	select {
	case req := <-requests:
		suite.Equal("/auth", req.path)
		suite.Equal("testservice", req.service)
		suite.Contains(req.scope, "repository:testrepo/local-subchart:pull")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("timeout waiting for auth request")
	}
}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
