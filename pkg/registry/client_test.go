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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
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
	// Bad bytes
	ref := fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost)
	_, err := suite.RegistryClient.Push([]byte("hello"), ref)
	suite.NotNil(err, "error pushing non-chart bytes")

	// Load a test chart
	chartData, err := ioutil.ReadFile("../repo/repotest/testdata/examplechart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")

	// non-strict ref (chart name)
	ref = fmt.Sprintf("%s/testrepo/boop:%s", suite.DockerRegistryHost, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref)
	suite.NotNil(err, "error pushing non-strict ref (bad basename)")

	// non-strict ref (chart name), with strict mode disabled
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptStrictMode(false))
	suite.Nil(err, "no error pushing non-strict ref (bad basename), with strict mode disabled")

	// non-strict ref (chart version)
	ref = fmt.Sprintf("%s/testrepo/%s:latest", suite.DockerRegistryHost, meta.Name)
	_, err = suite.RegistryClient.Push(chartData, ref)
	suite.NotNil(err, "error pushing non-strict ref (bad tag)")

	// non-strict ref (chart version), with strict mode disabled
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptStrictMode(false))
	suite.Nil(err, "no error pushing non-strict ref (bad tag), with strict mode disabled")

	// basic push, good ref
	chartData, err = ioutil.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err = extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref = fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref)
	suite.Nil(err, "no error pushing good ref")

	_, err = suite.RegistryClient.Pull(ref)
	suite.Nil(err, "no error pulling a simple chart")

	// Load another test chart
	chartData, err = ioutil.ReadFile("../downloader/testdata/signtest-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err = extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")

	// Load prov file
	provData, err := ioutil.ReadFile("../downloader/testdata/signtest-0.1.0.tgz.prov")
	suite.Nil(err, "no error loading test prov")

	// push with prov
	ref = fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	result, err := suite.RegistryClient.Push(chartData, ref, PushOptProvData(provData))
	suite.Nil(err, "no error pushing good ref with prov")

	_, err = suite.RegistryClient.Pull(ref)
	suite.Nil(err, "no error pulling a simple chart")

	// Validate the output
	// Note: these digests/sizes etc may change if the test chart/prov files are modified,
	// or if the format of the OCI manifest changes
	suite.Equal(ref, result.Ref)
	suite.Equal(meta.Name, result.Chart.Meta.Name)
	suite.Equal(meta.Version, result.Chart.Meta.Version)
	suite.Equal(int64(512), result.Manifest.Size)
	suite.Equal(int64(99), result.Config.Size)
	suite.Equal(int64(973), result.Chart.Size)
	suite.Equal(int64(695), result.Prov.Size)
	suite.Equal(
		"sha256:af4c20a1df1431495e673c14ecfa3a2ba24839a7784349d6787cd67957392e83",
		result.Manifest.Digest)
	suite.Equal(
		"sha256:8d17cb6bf6ccd8c29aace9a658495cbd5e2e87fc267876e86117c7db681c9580",
		result.Config.Digest)
	suite.Equal(
		"sha256:e5ef611620fb97704d8751c16bab17fedb68883bfb0edc76f78a70e9173f9b55",
		result.Chart.Digest)
	suite.Equal(
		"sha256:b0a02b7412f78ae93324d48df8fcc316d8482e5ad7827b5b238657a29a22f256",
		result.Prov.Digest)
}

func (suite *RegistryClientTestSuite) Test_2_Pull() {
	// bad/missing ref
	ref := fmt.Sprintf("%s/testrepo/no-existy:1.2.3", suite.DockerRegistryHost)
	_, err := suite.RegistryClient.Pull(ref)
	suite.NotNil(err, "error on bad/missing ref")

	// Load test chart (to build ref pushed in previous test)
	chartData, err := ioutil.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref = fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)

	// Simple pull, chart only
	_, err = suite.RegistryClient.Pull(ref)
	suite.Nil(err, "no error pulling a simple chart")

	// Simple pull with prov (no prov uploaded)
	_, err = suite.RegistryClient.Pull(ref, PullOptWithProv(true))
	suite.NotNil(err, "error pulling a chart with prov when no prov exists")

	// Simple pull with prov, ignoring missing prov
	_, err = suite.RegistryClient.Pull(ref,
		PullOptWithProv(true),
		PullOptIgnoreMissingProv(true))
	suite.Nil(err,
		"no error pulling a chart with prov when no prov exists, ignoring missing")

	// Load test chart (to build ref pushed in previous test)
	chartData, err = ioutil.ReadFile("../downloader/testdata/signtest-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err = extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref = fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)

	// Load prov file
	provData, err := ioutil.ReadFile("../downloader/testdata/signtest-0.1.0.tgz.prov")
	suite.Nil(err, "no error loading test prov")

	// no chart and no prov causes error
	_, err = suite.RegistryClient.Pull(ref,
		PullOptWithChart(false),
		PullOptWithProv(false))
	suite.NotNil(err, "error on both no chart and no prov")

	// full pull with chart and prov
	result, err := suite.RegistryClient.Pull(ref, PullOptWithProv(true))
	suite.Nil(err, "no error pulling a chart with prov")

	// Validate the output
	// Note: these digests/sizes etc may change if the test chart/prov files are modified,
	// or if the format of the OCI manifest changes
	suite.Equal(ref, result.Ref)
	suite.Equal(meta.Name, result.Chart.Meta.Name)
	suite.Equal(meta.Version, result.Chart.Meta.Version)
	suite.Equal(int64(512), result.Manifest.Size)
	suite.Equal(int64(99), result.Config.Size)
	suite.Equal(int64(973), result.Chart.Size)
	suite.Equal(int64(695), result.Prov.Size)
	suite.Equal(
		"sha256:af4c20a1df1431495e673c14ecfa3a2ba24839a7784349d6787cd67957392e83",
		result.Manifest.Digest)
	suite.Equal(
		"sha256:8d17cb6bf6ccd8c29aace9a658495cbd5e2e87fc267876e86117c7db681c9580",
		result.Config.Digest)
	suite.Equal(
		"sha256:e5ef611620fb97704d8751c16bab17fedb68883bfb0edc76f78a70e9173f9b55",
		result.Chart.Digest)
	suite.Equal(
		"sha256:b0a02b7412f78ae93324d48df8fcc316d8482e5ad7827b5b238657a29a22f256",
		result.Prov.Digest)
	suite.Equal("{\"schemaVersion\":2,\"config\":{\"mediaType\":\"application/vnd.cncf.helm.config.v1+json\",\"digest\":\"sha256:8d17cb6bf6ccd8c29aace9a658495cbd5e2e87fc267876e86117c7db681c9580\",\"size\":99},\"layers\":[{\"mediaType\":\"application/vnd.cncf.helm.chart.provenance.v1.prov\",\"digest\":\"sha256:b0a02b7412f78ae93324d48df8fcc316d8482e5ad7827b5b238657a29a22f256\",\"size\":695},{\"mediaType\":\"application/vnd.cncf.helm.chart.content.v1.tar+gzip\",\"digest\":\"sha256:e5ef611620fb97704d8751c16bab17fedb68883bfb0edc76f78a70e9173f9b55\",\"size\":973}]}",
		string(result.Manifest.Data))
	suite.Equal("{\"name\":\"signtest\",\"version\":\"0.1.0\",\"description\":\"A Helm chart for Kubernetes\",\"apiVersion\":\"v1\"}",
		string(result.Config.Data))
	suite.Equal(chartData, result.Chart.Data)
	suite.Equal(provData, result.Prov.Data)
}

func (suite *RegistryClientTestSuite) Test_3_Tags() {

	// Load test chart (to build ref pushed in previous test)
	chartData, err := ioutil.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s", suite.DockerRegistryHost, meta.Name)

	// Query for tags and validate length
	tags, err := suite.RegistryClient.Tags(ref)
	suite.Nil(err, "no error retrieving tags")
	suite.Equal(1, len(tags))

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

func initCompromisedRegistryTestServer() string {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "manifests") {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.WriteHeader(200)

			// layers[0] is the blob []byte("a")
			w.Write([]byte(
				fmt.Sprintf(`{ "schemaVersion": 2, "config": {
    "mediaType": "%s",
    "digest": "sha256:a705ee2789ab50a5ba20930f246dbd5cc01ff9712825bb98f57ee8414377f133",
    "size": 181
  },
  "layers": [
    {
      "mediaType": "%s",
      "digest": "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
      "size": 1
    }
  ]
}`, ConfigMediaType, ChartLayerMediaType)))
		} else if r.URL.Path == "/v2/testrepo/supposedlysafechart/blobs/sha256:a705ee2789ab50a5ba20930f246dbd5cc01ff9712825bb98f57ee8414377f133" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte("{\"name\":\"mychart\",\"version\":\"0.1.0\",\"description\":\"A Helm chart for Kubernetes\\n" +
				"an 'application' or a 'library' chart.\",\"apiVersion\":\"v2\",\"appVersion\":\"1.16.0\",\"type\":" +
				"\"application\"}"))
		} else if r.URL.Path == "/v2/testrepo/supposedlysafechart/blobs/sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
			w.Header().Set("Content-Type", ChartLayerMediaType)
			w.WriteHeader(200)
			w.Write([]byte("b"))
		} else {
			w.WriteHeader(500)
		}
	}))

	u, _ := url.Parse(s.URL)
	return fmt.Sprintf("localhost:%s", u.Port())
}

// Code from https://shaneutt.com/blog/golang-ca-and-signed-cert-go/
func genCerts(ip string) (caCert, caKey, clientCert, clientKey []byte, retErr error) {
	addr := net.ParseIP(ip)
	if addr == nil {
		retErr = fmt.Errorf("invalid IP %s", ip)
		return
	}
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			CommonName:   "helm.sh",
			Organization: []string{"Helm"},
			Country:      []string{"US"},
			Province:     []string{"CO"},
			Locality:     []string{"Boulder"},
		},
		IPAddresses:           []net.IP{addr},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create ca private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		retErr = err
		return
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		retErr = err
		return
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// client certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			Organization: []string{"Helm"},
			Country:      []string{"US"},
			Province:     []string{"CO"},
			Locality:     []string{"Boulder"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		retErr = err
		return
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		retErr = err
		return
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	caCert = caPEM.Bytes()
	caKey = caPrivKeyPEM.Bytes()
	clientCert = certPEM.Bytes()
	clientKey = certPrivKeyPEM.Bytes()

	return caCert, caKey, clientCert, clientKey, nil
}
