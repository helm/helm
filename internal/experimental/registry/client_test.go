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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"

	"helm.sh/helm/pkg/chart"
)

var (
	testCacheRootDir         = "helm-registry-test"
	testHtpasswdFileBasename = "authtest.htpasswd"
	testUsername             = "myuser"
	testPassword             = "mypass"
)

type RegistryClientTestSuite struct {
	suite.Suite
	Out                io.Writer
	DockerRegistryHost string
	CacheRootDir       string
	RegistryClient     *Client
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	suite.CacheRootDir = testCacheRootDir
	os.RemoveAll(suite.CacheRootDir)
	os.Mkdir(suite.CacheRootDir, 0700)

	var out bytes.Buffer
	suite.Out = &out
	credentialsFile := filepath.Join(suite.CacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	suite.Nil(err, "no error creating auth client")

	resolver, err := client.Resolver(context.Background())
	suite.Nil(err, "no error creating resolver")

	// create cache
	cache, err := NewCache(
		CacheOptDebug(true),
		CacheOptWriter(suite.Out),
		CacheOptRoot(filepath.Join(suite.CacheRootDir, CacheRootDir)),
	)
	suite.Nil(err, "no error creating cache")

	// init test client
	suite.RegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptWriter(suite.Out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptResolver(&Resolver{
			Resolver: resolver,
		}),
		ClientOptCache(cache),
	)
	suite.Nil(err, "no error creating registry client")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	suite.Nil(err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(suite.CacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	suite.Nil(err, "no error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := getFreePort()
	suite.Nil(err, "no error finding free port for test registry")
	suite.DockerRegistryHost = fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	suite.Nil(err, "no error creating test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	os.RemoveAll(suite.CacheRootDir)
}

func (suite *RegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost, "badverybad", "ohsobad")
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost, testUsername, testPassword)
	suite.Nil(err, "no error logging into registry with good credentials")
}

func (suite *RegistryClientTestSuite) Test_1_SaveChart() {
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)

	// empty chart
	err = suite.RegistryClient.SaveChart(&chart.Chart{}, ref)
	suite.NotNil(err)

	// valid chart
	ch := &chart.Chart{}
	ch.Metadata = &chart.Metadata{
		APIVersion: "v1",
		Name:       "testchart",
		Version:    "1.2.3",
	}
	err = suite.RegistryClient.SaveChart(ch, ref)
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_2_LoadChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	ch, err := suite.RegistryClient.LoadChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	ch, err = suite.RegistryClient.LoadChart(ref)
	suite.Nil(err)
	suite.Equal("testchart", ch.Metadata.Name)
	suite.Equal("1.2.3", ch.Metadata.Version)
}

func (suite *RegistryClientTestSuite) Test_3_PushChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.PushChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.PushChart(ref)
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_4_PullChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.PullChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.PullChart(ref)
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_5_PrintChartTable() {
	err := suite.RegistryClient.PrintChartTable()
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_6_RemoveChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.RemoveChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.RemoveChart(ref)
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_7_Logout() {
	err := suite.RegistryClient.Logout("this-host-aint-real:5000")
	suite.NotNil(err, "error logging out of registry that has no entry")

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")
}

func TestRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryClientTestSuite))
}

// borrowed from https://github.com/phayes/freeport
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
