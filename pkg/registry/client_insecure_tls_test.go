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

type InsecureTLSRegistryClientTestSuite struct {
	TestRegistry
}

func (suite *InsecureTLSRegistryClientTestSuite) SetupSuite() {
	// init test client
	setup(&suite.TestRegistry, true, true)
}

func (suite *InsecureTLSRegistryClientTestSuite) TearDownSuite() {
	teardown(&suite.TestRegistry)
	_ = os.RemoveAll(suite.WorkspaceDir)
}

func (suite *InsecureTLSRegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptInsecure(true))
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(true))
	suite.Nil(err, "no error logging into registry with good credentials")
}

func (suite *InsecureTLSRegistryClientTestSuite) Test_1_Push() {
	testPush(&suite.TestRegistry)
}

func (suite *InsecureTLSRegistryClientTestSuite) Test_2_Pull() {
	testPull(&suite.TestRegistry)
}

func (suite *InsecureTLSRegistryClientTestSuite) Test_3_Tags() {
	testTags(&suite.TestRegistry)
}

func (suite *InsecureTLSRegistryClientTestSuite) Test_4_Logout() {
	err := suite.RegistryClient.Logout("this-host-aint-real:5000")
	if err != nil {
		// credential backend for mac generates an error
		suite.NotNil(err, "failed to delete the credential for this-host-aint-real:5000")
	}

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")
}

func TestInsecureTLSRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(InsecureTLSRegistryClientTestSuite))
}
