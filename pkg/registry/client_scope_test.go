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

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull%2Cpush&service=testservice"), r.URL.String())
		w.WriteHeader(http.StatusOK)
	})
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.Nil(err, "no error creating server listner")

	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	// basic push, good ref
	testingChartCreationTime := "1977-09-02T22:04:05Z"
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptCreationTime(testingChartCreationTime))
	suite.NotNil(err, "error pushing good ref because auth server don't give proper token")

}

func (suite *RegistryScopeTestSuite) Test_2_Check_Pull_Request_Scope() {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull&service=testservice"), r.URL.String())
		w.WriteHeader(http.StatusOK)
	})
	listener, err := net.Listen("tcp", suite.AuthServerHost)
	suite.Nil(err, "no error creating server listner")

	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	// Load test chart (to build ref pushed in previous test)
	// Simple pull, chart only
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Pull(ref)
	suite.NotNil(err, "error pulling a simple chart because auth server don't give proper token")

}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
