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
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistryScopeTestSuite struct {
	TestRegistry

	// authServer serves the token realm for the whole suite on the listener
	// reserved by setup(). It is closed in TearDownSuite, which releases the
	// reserved port only once every test in the suite has finished.
	authServer *httptest.Server
	authMu     sync.Mutex
	authURLs   []string
}

// handleAuth records the requested URL and replies without a token payload,
// so the push/pull flow fails, which is what the tests assert on. It never
// blocks, so the auth server always responds and the push/pull flow cannot
// deadlock waiting on us.
func (suite *RegistryScopeTestSuite) handleAuth(w http.ResponseWriter, r *http.Request) {
	suite.authMu.Lock()
	suite.authURLs = append(suite.authURLs, r.URL.String())
	suite.authMu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (suite *RegistryScopeTestSuite) recordedAuthURLs() []string {
	suite.authMu.Lock()
	defer suite.authMu.Unlock()
	return append([]string(nil), suite.authURLs...)
}

func (suite *RegistryScopeTestSuite) SetupSuite() {
	// Set up a plain-HTTP registry that uses token auth. The token realm is
	// served over http (see setup), so the registry must be contacted over
	// http as well: oras refuses to send credentials to an http token realm
	// when the registry itself was reached over https.
	setup(&suite.TestRegistry, false, false, "token")

	// Start the realm server once for the whole suite, directly on the
	// listener that setup() reserved. Re-binding the same address per test
	// (the previous behaviour) left a window between setup()'s probe close
	// and the test's Listen call in which any other process could steal the
	// port, so the token realm could silently belong to a stranger.
	suite.authServer = httptest.NewUnstartedServer(http.HandlerFunc(suite.handleAuth))
	suite.authServer.Listener = suite.authListener
	suite.authServer.Start()
}

func (suite *RegistryScopeTestSuite) TearDownSuite() {
	if suite.authServer != nil {
		suite.authServer.Close()
		suite.authServer = nil
	}
	teardown(&suite.TestRegistry)
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *RegistryScopeTestSuite) Test_0_Auth_Port_Stays_Reserved() {
	// Regression guard for the free-port re-bind race: setup() used to close
	// the auth probe listener when it returned, leaving the port free until
	// each test re-bound it. If any other process won that window, the token
	// realm silently belonged to the stranger. The suite must hold the
	// listener until teardown, so binding the same address must fail.
	ln, err := net.Listen("tcp", suite.AuthServerHost)
	if err == nil {
		_ = ln.Close()
		suite.T().Fatal("auth server port must stay reserved by the suite until teardown")
	}
}

func (suite *RegistryScopeTestSuite) Test_1_Check_Push_Request_Scope() {
	before := len(suite.recordedAuthURLs())

	// basic push, good ref
	testingChartCreationTime := "1977-09-02T22:04:05Z"
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Require().NoError(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Require().NoError(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptCreationTime(testingChartCreationTime))
	suite.Require().Error(err, "error pushing good ref because auth server doesn't give proper token")

	// check the url that authentication server received
	urls := suite.recordedAuthURLs()
	if len(urls) <= before {
		suite.T().Fatal("auth server did not receive any request during push")
	}
	u, err := url.Parse(urls[before])
	suite.Require().NoError(err, "no error parsing requested URL")

	suite.Equal("/auth", u.Path)
	suite.Equal("testservice", u.Query().Get("service"))
	scope := u.Query().Get("scope")
	suite.Contains(scope, "repository:testrepo/local-subchart:pull,push")
}

func (suite *RegistryScopeTestSuite) Test_2_Check_Pull_Request_Scope() {
	before := len(suite.recordedAuthURLs())

	// Load test chart (to build ref pushed in previous test)
	// Simple pull, chart only
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Require().NoError(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Require().NoError(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Pull(ref)
	suite.Require().Error(err, "error pulling a simple chart because auth server doesn't give proper token")

	// check the url that authentication server received
	urls := suite.recordedAuthURLs()
	if len(urls) <= before {
		suite.T().Fatal("auth server did not receive any request during pull")
	}
	u, err := url.Parse(urls[before])
	suite.Require().NoError(err, "no error parsing requested URL")

	suite.Equal("/auth", u.Path)
	suite.Equal("testservice", u.Query().Get("service"))
	scope := u.Query().Get("scope")
	suite.Contains(scope, "repository:testrepo/local-subchart:pull")
}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
