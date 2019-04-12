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
	"net"
	"os"
	"testing"
	"time"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/stretchr/testify/suite"

	"helm.sh/helm/pkg/chart"
)

var (
	testCacheRootDir = "helm-registry-test"
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

	// Init test client
	var out bytes.Buffer
	suite.Out = &out
	suite.RegistryClient = NewClient(&ClientOptions{
		Out: suite.Out,
		Resolver: Resolver{
			Resolver: docker.NewResolver(docker.ResolverOptions{}),
		},
		CacheRootDir: suite.CacheRootDir,
	})

	// Registry config
	config := &configuration.Configuration{}
	port, err := getFreePort()
	if err != nil {
		suite.Nil(err, "no error finding free port for test registry")
	}
	suite.DockerRegistryHost = fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	suite.Nil(err, "no error creating test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	os.RemoveAll(suite.CacheRootDir)
}

func (suite *RegistryClientTestSuite) Test_0_SaveChart() {
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

func (suite *RegistryClientTestSuite) Test_1_LoadChart() {

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

func (suite *RegistryClientTestSuite) Test_2_PushChart() {

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

func (suite *RegistryClientTestSuite) Test_3_PullChart() {

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

func (suite *RegistryClientTestSuite) Test_4_PrintChartTable() {
	err := suite.RegistryClient.PrintChartTable()
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_5_RemoveChart() {

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
