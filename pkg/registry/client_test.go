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

	"github.com/containerd/containerd/errdefs"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

var (
	testWorkspaceDir         = "helm-registry-test"
	testHtpasswdFileBasename = "authtest.htpasswd"
	testCACertFileName       = "root.pem"
	testCAKeyFileName        = "root-key.pem"
	testClientCertFileName   = "client.pem"
	testClientKeyFileName    = "client-key.pem"
	testUsername             = "myuser"
	testPassword             = "mypass"
)

type RegistryClientTestSuite struct {
	suite.Suite
	Out                     io.Writer
	DockerRegistryHost      string
	CompromisedRegistryHost string
	WorkspaceDir            string
	RegistryClient          *Client

	PlainHTTPDockerRegistryHost       string
	TLSDockerRegistryHost             string
	TLSVerifyClientDockerRegistryHost string

	PlainHTTPRegistryClient   *Client
	InsecureRegistryClient    *Client
	RegistryClientWithCA      *Client
	RegistryClientWithCertKey *Client
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	suite.WorkspaceDir = testWorkspaceDir
	os.RemoveAll(suite.WorkspaceDir)
	os.Mkdir(suite.WorkspaceDir, 0700)

	var out bytes.Buffer
	suite.Out = &out
	credentialsFile := filepath.Join(suite.WorkspaceDir, CredentialsFileBasename)

	// find the first non-local IP as the registry address
	// or else, using localhost will always be insecure
	var hostname string
	addrs, err := net.InterfaceAddrs()
	suite.Nil(err, "error getting IP addresses")
	for _, address := range addrs {
		if n, ok := address.(*net.IPNet); ok {
			if n.IP.IsLinkLocalUnicast() || n.IP.IsLoopback() {
				continue
			}
			hostname = n.IP.String()
			break
		}
	}
	suite.NotEmpty(hostname, "failed to get ip address as hostname")

	// generate self-sign CA cert/key and client cert/key
	caCert, caKey, clientCert, clientKey, err := genCerts(hostname)
	suite.Nil(err, "error generating certs")
	caCertPath := filepath.Join(suite.WorkspaceDir, testCACertFileName)
	err = ioutil.WriteFile(caCertPath, caCert, 0644)
	suite.Nil(err, "error creating test ca cert file")
	caKeyPath := filepath.Join(suite.WorkspaceDir, testCAKeyFileName)
	err = ioutil.WriteFile(caKeyPath, caKey, 0644)
	suite.Nil(err, "error creating test ca key file")
	clientCertPath := filepath.Join(suite.WorkspaceDir, testClientCertFileName)
	err = ioutil.WriteFile(clientCertPath, clientCert, 0644)
	suite.Nil(err, "error creating test client cert file")
	clientKeyPath := filepath.Join(suite.WorkspaceDir, testClientKeyFileName)
	err = ioutil.WriteFile(clientKeyPath, clientKey, 0644)
	suite.Nil(err, "error creating test client key file")

	// init test client
	suite.RegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptEnableCache(true),
		ClientOptWriter(suite.Out),
		ClientOptCredentialsFile(credentialsFile),
	)
	suite.Nil(err, "no error creating registry client")

	// init plain http client
	suite.PlainHTTPRegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptEnableCache(true),
		ClientOptWriter(suite.Out),
		ClientOptCredentialsFile(credentialsFile),
		ClientOptPlainHTTP(true),
	)
	suite.Nil(err, "error creating plain http registry client")

	// init insecure client
	suite.InsecureRegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptEnableCache(true),
		ClientOptWriter(suite.Out),
		ClientOptInsecureSkipVerifyTLS(true),
	)
	suite.Nil(err, "error creating insecure registry client")

	// init client with CA cert
	suite.RegistryClientWithCA, err = NewClient(
		ClientOptDebug(true),
		ClientOptEnableCache(true),
		ClientOptWriter(suite.Out),
		ClientOptCAFile(caCertPath),
	)
	suite.Nil(err, "error creating registry client with CA cert")

	// init client with CA cert and client cert/key
	suite.RegistryClientWithCertKey, err = NewClient(
		ClientOptDebug(true),
		ClientOptEnableCache(true),
		ClientOptWriter(suite.Out),
		ClientOptCAFile(caCertPath),
		ClientOptCertKeyFiles(clientCertPath, clientKeyPath),
	)
	suite.Nil(err, "error creating registry client with CA cert")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	suite.Nil(err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(suite.WorkspaceDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	suite.Nil(err, "no error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test registry")
	suite.DockerRegistryHost = fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf("127.0.0.1:%d", port)
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

	suite.CompromisedRegistryHost = initCompromisedRegistryTestServer()

	// plain http registry
	plainHTTPConfig := &configuration.Configuration{}
	plainHTTPPort, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test plain HTTP registry")
	suite.PlainHTTPDockerRegistryHost = fmt.Sprintf("%s:%d", hostname, plainHTTPPort)
	plainHTTPConfig.HTTP.Addr = fmt.Sprintf(":%d", plainHTTPPort)
	plainHTTPConfig.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	plainHTTPConfig.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": hostname,
			"path":  htpasswdPath,
		},
	}
	plainHTTPDockerRegistry, err := registry.NewRegistry(context.Background(), plainHTTPConfig)
	suite.Nil(err, "no error creating test plain http registry")

	// init TLS registry with self-signed CA
	tlsRegistryPort, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test TLS registry")
	suite.TLSDockerRegistryHost = fmt.Sprintf("%s:%d", hostname, tlsRegistryPort)

	tlsRegistryConfig := &configuration.Configuration{}
	tlsRegistryConfig.HTTP.Addr = fmt.Sprintf(":%d", tlsRegistryPort)
	tlsRegistryConfig.HTTP.TLS.Certificate = caCertPath
	tlsRegistryConfig.HTTP.TLS.Key = caKeyPath
	tlsRegistryConfig.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	tlsRegistryConfig.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": hostname,
			"path":  htpasswdPath,
		},
	}
	tlsDockerRegistry, err := registry.NewRegistry(context.Background(), tlsRegistryConfig)
	suite.Nil(err, "no error creating test TLS registry")
	// init TLS registry with self-signed CA and client verification enabled
	anotherTLSRegistryPort, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test another TLS registry")
	suite.TLSVerifyClientDockerRegistryHost = fmt.Sprintf("%s:%d", hostname, anotherTLSRegistryPort)

	anotherTLSRegistryConfig := &configuration.Configuration{}
	anotherTLSRegistryConfig.HTTP.Addr = fmt.Sprintf(":%d", anotherTLSRegistryPort)
	anotherTLSRegistryConfig.HTTP.TLS.Certificate = caCertPath
	anotherTLSRegistryConfig.HTTP.TLS.Key = caKeyPath
	anotherTLSRegistryConfig.HTTP.TLS.ClientCAs = []string{caCertPath}
	anotherTLSRegistryConfig.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	// no auth because we cannot pass Login action
	anotherTLSDockerRegistry, err := registry.NewRegistry(context.Background(), anotherTLSRegistryConfig)
	suite.Nil(err, "no error creating test another TLS registry")

	// start registries
	go dockerRegistry.ListenAndServe()
	go plainHTTPDockerRegistry.ListenAndServe()
	go tlsDockerRegistry.ListenAndServe()
	go anotherTLSDockerRegistry.ListenAndServe()
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *RegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptInsecure(false))
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth("badverybad", "ohsobad"),
		LoginOptInsecure(true))
	suite.NotNil(err, "error logging into registry with bad credentials, insecure mode")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(false))
	suite.Nil(err, "no error logging into registry with good credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(true))
	suite.Nil(err, "no error logging into registry with good credentials, insecure mode")

	err = suite.PlainHTTPRegistryClient.Login(suite.PlainHTTPDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(false))
	suite.NotNil(err, "no error logging into registry with good credentials")

	err = suite.PlainHTTPRegistryClient.Login(suite.PlainHTTPDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(true))
	suite.Nil(err, "error logging into registry with good credentials, insecure mode")

	err = suite.InsecureRegistryClient.Login(suite.TLSDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(false))
	suite.NotNil(err, "no error logging into insecure with good credentials")

	err = suite.InsecureRegistryClient.Login(suite.TLSDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(true))
	suite.Nil(err, "error logging into insecure with good credentials, insecure mode")

	err = suite.RegistryClientWithCA.Login(suite.TLSDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(false))
	suite.NotNil(err, "no error logging into insecure with good credentials")

	err = suite.RegistryClientWithCA.Login(suite.TLSDockerRegistryHost,
		LoginOptBasicAuth(testUsername, testPassword),
		LoginOptInsecure(true))
	suite.Nil(err, "error logging into insecure with good credentials, insecure mode")
}

func (suite *RegistryClientTestSuite) Test_1_Push() {
	testPush(&suite.TestSuite)
}

func (suite *RegistryClientTestSuite) Test_2_Pull() {
	testPull(&suite.TestSuite)
}

func (suite *RegistryClientTestSuite) Test_3_Tags() {
	testTags(&suite.TestSuite)
}

func (suite *RegistryClientTestSuite) Test_4_Logout() {
	err := suite.RegistryClient.Logout("this-host-aint-real:5000")
	suite.NotNil(err, "error logging out of registry that has no entry")

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")

	err = suite.PlainHTTPRegistryClient.Logout(suite.PlainHTTPDockerRegistryHost)
	suite.Nil(err, "error logging out of plain http registry")

	err = suite.InsecureRegistryClient.Logout(suite.TLSDockerRegistryHost)
	suite.Nil(err, "error logging out of insecure registry")

	// error as logout happened for TLSDockerRegistryHost in last step
	err = suite.RegistryClientWithCA.Logout(suite.TLSDockerRegistryHost)
	suite.Nil(err, "no error logging out of registry with ca cert")

}

func (suite *RegistryClientTestSuite) Test_5_ManInTheMiddle() {
	ref := fmt.Sprintf("%s/testrepo/supposedlysafechart:9.9.9", suite.CompromisedRegistryHost)

	// returns content that does not match the expected digest
	_, err := suite.RegistryClient.Pull(ref)
	suite.NotNil(err)
	suite.True(errdefs.IsFailedPrecondition(err))
}

func TestRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryClientTestSuite))
}
