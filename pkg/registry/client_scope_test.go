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
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type RegistryScopeTestSuite struct {
	TestRegistry
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

	requestURL := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL <- r.URL.String()
		w.WriteHeader(http.StatusOK)
	})
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.NoError(err, "no error creating server listener")

	ts := httptest.NewUnstartedServer(handler)
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

	//check the url that authentication server received
	select {
	case urlStr := <-requestURL:
		u, err := url.Parse(urlStr)
		suite.NoError(err, "no error parsing requested URL")

		suite.Equal("/auth", u.Path)
		suite.Equal("testservice", u.Query().Get("service"))
		scope := u.Query().Get("scope")
		suite.Contains(scope, "repository:testrepo/local-subchart:pull,push")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("timeout waiting for auth request")
	}
}

func (suite *RegistryScopeTestSuite) Test_2_Check_Pull_Request_Scope() {

	requestURL := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL <- r.URL.String()
		w.WriteHeader(http.StatusOK)
	})
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.NoError(err, "no error creating server listener")

	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	// Load test chart (to build ref pushed in previous test)
	// Simple pull, chart only
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.NoError(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.NoError(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Pull(ref)
	suite.Error(err, "error pulling a simple chart because auth server doesn't give proper token")

	//check the url that authentication server received
	select {
	case urlStr := <-requestURL:
		u, err := url.Parse(urlStr)
		suite.NoError(err, "no error parsing requested URL")

		suite.Equal("/auth", u.Path)
		suite.Equal("testservice", u.Query().Get("service"))
		scope := u.Query().Get("scope")
		suite.Contains(scope, "repository:testrepo/local-subchart:pull")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("timeout waiting for auth request")
	}
}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
