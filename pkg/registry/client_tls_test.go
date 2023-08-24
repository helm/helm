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
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TLSRegistryClientTestSuite struct {
	TestSuite
}

func (suite *TLSRegistryClientTestSuite) SetupSuite() {
	// init test client
	dockerRegistry := setup(&suite.TestSuite, true, false)

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *TLSRegistryClientTestSuite) TearDownSuite() {
	teardown(&suite.TestSuite)
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *TLSRegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptTLSClientConfig(tlsCert, tlsKey, tlsCA))
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptTLSClientConfig(tlsCert, tlsKey, tlsCA))
	suite.Nil(err, "no error logging into registry with good credentials")
}

func (suite *TLSRegistryClientTestSuite) Test_1_Push() {
	testPush(&suite.TestSuite)
}

func (suite *TLSRegistryClientTestSuite) Test_2_Pull() {
	testPull(&suite.TestSuite)
}

func (suite *TLSRegistryClientTestSuite) Test_3_Tags() {
	testTags(&suite.TestSuite)
}

func (suite *TLSRegistryClientTestSuite) Test_4_Logout() {
	err := suite.RegistryClient.Logout("this-host-aint-real:5000")
	suite.NotNil(err, "error logging out of registry that has no entry")

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")
}

func TestTLSRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(TLSRegistryClientTestSuite))
}
