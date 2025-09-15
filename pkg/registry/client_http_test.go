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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/content"
)

type HTTPRegistryClientTestSuite struct {
	TestSuite
}

func (suite *HTTPRegistryClientTestSuite) SetupSuite() {
	// init test client
	setup(&suite.TestSuite, false, false)
}

func (suite *HTTPRegistryClientTestSuite) TearDownSuite() {
	teardown(&suite.TestSuite)
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *HTTPRegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptPlainText(true))
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptPlainText(true))
	suite.Nil(err, "no error logging into registry with good credentials")
}

func (suite *HTTPRegistryClientTestSuite) Test_1_Push() {
	testPush(&suite.TestSuite)
}

func (suite *HTTPRegistryClientTestSuite) Test_2_Pull() {
	testPull(&suite.TestSuite)
}

func (suite *HTTPRegistryClientTestSuite) Test_3_Tags() {
	testTags(&suite.TestSuite)
}

func (suite *HTTPRegistryClientTestSuite) Test_4_ManInTheMiddle() {
	ref := fmt.Sprintf("%s/testrepo/supposedlysafechart:9.9.9", suite.CompromisedRegistryHost)

	// returns content that does not match the expected digest
	_, err := suite.RegistryClient.Pull(ref)
	suite.NotNil(err)
	suite.True(errors.Is(err, content.ErrMismatchedDigest))
}

func TestHTTPRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPRegistryClientTestSuite))
}
