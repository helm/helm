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
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TLSRegistryClientTestSuite struct {
	TestRegistry
}

func (suite *TLSRegistryClientTestSuite) SetupSuite() {
	// init test client
	setup(&suite.TestRegistry, true, false)
}

func (suite *TLSRegistryClientTestSuite) TearDownSuite() {
	teardown(&suite.TestRegistry)
	_ = os.RemoveAll(suite.WorkspaceDir)
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

func (suite *TLSRegistryClientTestSuite) Test_1_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptTLSClientConfigFromConfig(&tls.Config{}))
	suite.NotNil(err, "error logging into registry with bad credentials")

	// Create a *tls.Config from tlsCert, tlsKey, and tlsCA.
	cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	suite.Nil(err, "error loading x509 key pair")
	rootCAs := x509.NewCertPool()
	caCert, err := os.ReadFile(tlsCA)
	suite.Nil(err, "error reading CA certificate")
	rootCAs.AppendCertsFromPEM(caCert)
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
	}

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptTLSClientConfigFromConfig(conf))
	suite.Nil(err, "no error logging into registry with good credentials")
}

func (suite *TLSRegistryClientTestSuite) Test_1_Push() {
	testPush(&suite.TestRegistry)
}

func (suite *TLSRegistryClientTestSuite) Test_2_Pull() {
	testPull(&suite.TestRegistry)
}

func (suite *TLSRegistryClientTestSuite) Test_3_Tags() {
	testTags(&suite.TestRegistry)
}

func (suite *TLSRegistryClientTestSuite) Test_4_Logout() {
	err := suite.RegistryClient.Logout("this-host-aint-real:5000")
	if err != nil {
		// credential backend for mac generates an error
		suite.NotNil(err, "failed to delete the credential for this-host-aint-real:5000")
	}

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")
}

func TestTLSRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(TLSRegistryClientTestSuite))
}
