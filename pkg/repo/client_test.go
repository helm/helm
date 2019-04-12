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

package repo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/containerd/containerd/reference"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/stretchr/testify/suite"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/repo/repotest"
)

type RegistryClientTestSuite struct {
	suite.Suite
	DockerRegistryHost string
	RegistryClient     *Client
	CacheDir           string
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	var err error
	suite.CacheDir, err = ioutil.TempDir("", "helm-registry-test")
	suite.Nil(err)

	// Init test client
	suite.RegistryClient = NewClient(&ClientOptions{
		Out:          ioutil.Discard,
		CacheRootDir: suite.CacheDir,
	})

	testRepo := repotest.NewServer()
	suite.DockerRegistryHost = testRepo.URL()
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	suite.Nil(os.RemoveAll(suite.CacheDir))
}

func (suite *RegistryClientTestSuite) Test_0_SaveChart() {
	// empty chart
	suite.NotNil(suite.RegistryClient.SaveChart(&chart.Chart{}, suite.DockerRegistryHost))

	// valid chart
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
			Name:       "testchart",
			Version:    "1.2.3",
		},
	}

	suite.Nil(suite.RegistryClient.SaveChart(ch, suite.DockerRegistryHost))

	ch2 := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
			Name:       "testchart",
			Version:    "4.5.6",
		},
	}

	suite.Nil(suite.RegistryClient.SaveChart(ch2, suite.DockerRegistryHost))
}

func (suite *RegistryClientTestSuite) Test_1_LoadChart() {

	// non-existent ref
	ref, err := reference.Parse(path.Join(suite.DockerRegistryHost, "whodis:9.9.9"))
	suite.Nil(err)
	ch, err := suite.RegistryClient.LoadChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:1.2.3"))
	suite.Nil(err)
	ch, err = suite.RegistryClient.LoadChart(ref)
	suite.Nil(err)
	suite.Equal("testchart", ch.Metadata.Name)
	suite.Equal("1.2.3", ch.Metadata.Version)

	// existing ref
	ref2, err := reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:4.5.6"))
	suite.Nil(err)
	ch, err = suite.RegistryClient.LoadChart(ref2)
	suite.Nil(err)
	suite.Equal("testchart", ch.Metadata.Name)
	suite.Equal("4.5.6", ch.Metadata.Version)
}

func (suite *RegistryClientTestSuite) Test_2_PushChart() {

	// non-existent ref
	ref, err := reference.Parse(fmt.Sprintf("%s/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	suite.NotNil(suite.RegistryClient.PushChart(ref))

	// existing ref
	ref, err = reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:1.2.3"))
	suite.Nil(err)
	suite.Nil(suite.RegistryClient.PushChart(ref))

	ref2, err := reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:4.5.6"))
	suite.Nil(err)
	suite.Nil(suite.RegistryClient.PushChart(ref2))
}

func (suite *RegistryClientTestSuite) Test_3_PullChart() {

	// non-existent ref
	ref, err := reference.Parse(fmt.Sprintf("%s/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	suite.NotNil(suite.RegistryClient.PullChart(ref))

	// existing ref
	ref, err = reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:1.2.3"))
	suite.Nil(err)
	suite.Nil(suite.RegistryClient.PullChart(ref))
}

func (suite *RegistryClientTestSuite) Test_4_PrintChartTable() {
	err := suite.RegistryClient.PrintChartTable()
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_5_RemoveChart() {

	// non-existent ref
	ref, err := reference.Parse(fmt.Sprintf("%s/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.RemoveChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:1.2.3"))
	suite.Nil(err)
	err = suite.RegistryClient.RemoveChart(ref)
	suite.Nil(err)
}

func TestRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryClientTestSuite))
}

func (suite *RegistryClientTestSuite) Test_6_FindChart() {
	// previous tests saved and pushed this chart
	ref, err := reference.Parse(path.Join(suite.DockerRegistryHost, "testchart:1.2.3"))
	suite.Nil(err)

	if err := suite.RegistryClient.FindChart(ref); err != nil {
		suite.T().Error(err)
	}
}

func (suite *RegistryClientTestSuite) Test_7_FindChartError() {
	ref, err := reference.Parse("someserver.local/something/nginx")
	suite.Nil(err)

	ref2, err := reference.Parse(path.Join(suite.DockerRegistryHost, "nginx1"))
	suite.Nil(err)

	err = suite.RegistryClient.FindChart(ref)
	if err == nil {
		suite.T().Errorf("Expected error for bad chart URL, but did not get any errors")
	}
	if err != nil && err.Error() != `chart "someserver.local/something/nginx" not found in repository` {
		suite.T().Errorf("Expected error for bad chart URL, but got a different error (%v)", err)
	}

	err = suite.RegistryClient.FindChart(ref2)
	if err == nil {
		suite.T().Errorf("Expected error for chart not found, but did not get any errors")
	}
	if err != nil && err.Error() != `chart "`+suite.DockerRegistryHost+`/nginx1" not found in repository` {
		suite.T().Errorf("Expected error for chart not found, but got a different error (%v)", err)
	}
}

func (suite *RegistryClientTestSuite) Test_8_FetchTags() {
	tags, err := suite.RegistryClient.FetchTags(path.Join(suite.DockerRegistryHost, "testchart"))
	suite.Nil(err)
	fmt.Println(tags)
	suite.ElementsMatch(tags, []string{"1.2.3", "4.5.6"})
}
