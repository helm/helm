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
	"context"
	"fmt"
	"log"
	"net/http"
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

	//set simple auth server to check the auth request scope
	server := &http.Server{
		Addr: suite.AuthServerHost,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			suite.Equal(string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull%2Cpush&service=testservice"), r.URL.String())
			w.WriteHeader(http.StatusOK)
		}),
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed to ListenAndServe:%v", err)
		}
	}()

	// basic push, good ref
	testingChartCreationTime := "1977-09-02T22:04:05Z"
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptCreationTime(testingChartCreationTime))
	suite.NotNil(err, "error pushing good ref because auth server don't give proper token")

	//shutdown auth server
	err = server.Shutdown(context.Background())
	suite.Nil(err, "shutdown simple auth server")

}

func (suite *RegistryScopeTestSuite) Test_2_Check_Pull_Request_Scope() {

	//set simple auth server to check the auth request scope
	server := &http.Server{
		Addr: suite.AuthServerHost,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			suite.Equal(string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull&service=testservice"), r.URL.String())
			w.WriteHeader(http.StatusOK)
		}),
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed to ListenAndServe:%v", err)
		}
	}()

	// Load test chart (to build ref pushed in previous test)
	// Simple pull, chart only
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Pull(ref)
	suite.NotNil(err, "error pulling a simple chart because auth server don't give proper token")

	//shutdown auth server
	err = server.Shutdown(context.Background())
	suite.Nil(err, "shutdown simple auth server")
}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
