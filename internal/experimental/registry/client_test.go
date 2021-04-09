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
	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"

	"helm.sh/helm/v3/pkg/chart"
)

var (
	testCacheRootDir         = "helm-registry-test"
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
	CacheRootDir            string
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
	suite.CacheRootDir = testCacheRootDir
	os.RemoveAll(suite.CacheRootDir)
	os.Mkdir(suite.CacheRootDir, 0700)

	var out bytes.Buffer
	suite.Out = &out
	credentialsFile := filepath.Join(suite.CacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	suite.Nil(err, "error creating auth client")

	resolver, err := client.Resolver(context.Background(), http.DefaultClient, false)
	suite.Nil(err, "no error creating resolver")

	// create cache
	cache, err := NewCache(
		CacheOptDebug(true),
		CacheOptWriter(suite.Out),
		CacheOptRoot(filepath.Join(suite.CacheRootDir, CacheRootDir)),
	)
	suite.Nil(err, "error creating cache")

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
	caCertPath := filepath.Join(suite.CacheRootDir, testCACertFileName)
	err = ioutil.WriteFile(caCertPath, caCert, 0644)
	suite.Nil(err, "error creating test ca cert file")
	caKeyPath := filepath.Join(suite.CacheRootDir, testCAKeyFileName)
	err = ioutil.WriteFile(caKeyPath, caKey, 0644)
	suite.Nil(err, "error creating test ca key file")
	clientCertPath := filepath.Join(suite.CacheRootDir, testClientCertFileName)
	err = ioutil.WriteFile(clientCertPath, clientCert, 0644)
	suite.Nil(err, "error creating test client cert file")
	clientKeyPath := filepath.Join(suite.CacheRootDir, testClientKeyFileName)
	err = ioutil.WriteFile(clientKeyPath, clientKey, 0644)
	suite.Nil(err, "error creating test client key file")

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

	// init plain http client
	suite.PlainHTTPRegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptWriter(suite.Out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptPlainHTTP(true),
		ClientOptCache(cache),
	)
	suite.Nil(err, "error creating plain http registry client")

	// init insecure client
	suite.InsecureRegistryClient, err = NewClient(
		ClientOptDebug(true),
		ClientOptWriter(suite.Out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptInsecureSkipVerifyTLS(true),
		ClientOptCache(cache),
	)
	suite.Nil(err, "error creating insecure registry client")

	// init client with CA cert
	suite.RegistryClientWithCA, err = NewClient(
		ClientOptDebug(true),
		ClientOptWriter(suite.Out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptCAFile(caCertPath),
		ClientOptCache(cache),
	)
	suite.Nil(err, "error creating registry client with CA cert")

	// init client with CA cert and client cert/key
	suite.RegistryClientWithCertKey, err = NewClient(
		ClientOptDebug(true),
		ClientOptWriter(suite.Out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptCAFile(caCertPath),
		ClientOptCertKeyFiles(clientCertPath, clientKeyPath),
		ClientOptCache(cache),
	)
	suite.Nil(err, "error creating registry client with CA cert")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	suite.Nil(err, "error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(suite.CacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	suite.Nil(err, "error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test plain HTTP registry")
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
	os.RemoveAll(suite.CacheRootDir)
}

func (suite *RegistryClientTestSuite) Test_0_Login() {
	err := suite.RegistryClient.Login(suite.DockerRegistryHost, "badverybad", "ohsobad", false)
	suite.NotNil(err, "error logging into registry with bad credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost, "badverybad", "ohsobad", true)
	suite.NotNil(err, "error logging into registry with bad credentials, insecure mode")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost, testUsername, testPassword, false)
	suite.Nil(err, "no error logging into registry with good credentials")

	err = suite.RegistryClient.Login(suite.DockerRegistryHost, testUsername, testPassword, true)
	suite.Nil(err, "no error logging into registry with good credentials, insecure mode")

	err = suite.PlainHTTPRegistryClient.Login(suite.PlainHTTPDockerRegistryHost, testUsername, testPassword, false)
	suite.NotNil(err, "no error logging into registry with good credentials")

	err = suite.PlainHTTPRegistryClient.Login(suite.PlainHTTPDockerRegistryHost, testUsername, testPassword, true)
	suite.Nil(err, "error logging into registry with good credentials, insecure mode")

	err = suite.InsecureRegistryClient.Login(suite.TLSDockerRegistryHost, testUsername, testPassword, false)
	suite.NotNil(err, "no error logging into insecure with good credentials")

	err = suite.InsecureRegistryClient.Login(suite.TLSDockerRegistryHost, testUsername, testPassword, true)
	suite.Nil(err, "error logging into insecure with good credentials, insecure mode")

	err = suite.RegistryClientWithCA.Login(suite.TLSDockerRegistryHost, testUsername, testPassword, false)
	suite.NotNil(err, "no error logging into insecure with good credentials")

	err = suite.RegistryClientWithCA.Login(suite.TLSDockerRegistryHost, testUsername, testPassword, true)
	suite.Nil(err, "error logging into insecure with good credentials, insecure mode")
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

	// prepare the cache for plain http/TLS registries
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.PlainHTTPDockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.SaveChart(ch, ref)
	suite.Nil(err)

	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSDockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.SaveChart(ch, ref)
	suite.Nil(err)

	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSVerifyClientDockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClient.SaveChart(ch, ref)
	suite.Nil(err)
}

func (suite *RegistryClientTestSuite) Test_2_LoadChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	_, err = suite.RegistryClient.LoadChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	ch, err := suite.RegistryClient.LoadChart(ref)
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

	// plain http registry
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.PlainHTTPDockerRegistryHost))
	suite.Nil(err)
	err = suite.PlainHTTPRegistryClient.PushChart(ref)
	suite.Nil(err)

	// insecure TLS registry
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSDockerRegistryHost))
	suite.Nil(err)
	err = suite.InsecureRegistryClient.PushChart(ref)
	suite.Nil(err, "error insecure pushing %s", ref.FullName())

	// TLS registry with CA cert
	err = suite.RegistryClientWithCA.PushChart(ref)
	suite.Nil(err, "error pushing %s  with CA", ref.FullName())

	// TLS registry with client cert verification enabled
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSVerifyClientDockerRegistryHost))
	suite.Nil(err)
	err = suite.RegistryClientWithCertKey.PushChart(ref)
	suite.Nil(err, "error pushing %s with client cert", ref.FullName())
}

func (suite *RegistryClientTestSuite) Test_4_PullChart() {

	// non-existent ref
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/whodis:9.9.9", suite.DockerRegistryHost))
	suite.Nil(err)
	_, err = suite.RegistryClient.PullChart(ref)
	suite.NotNil(err)

	// existing ref
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.DockerRegistryHost))
	suite.Nil(err)
	_, err = suite.RegistryClient.PullChart(ref)
	suite.Nil(err)

	// plain http registry
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.PlainHTTPDockerRegistryHost))
	suite.Nil(err)
	_, err = suite.PlainHTTPRegistryClient.PullChart(ref)
	suite.Nil(err)

	// insecure registry
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSDockerRegistryHost))
	suite.Nil(err)
	_, err = suite.InsecureRegistryClient.PullChart(ref)
	suite.Nil(err)

	// registry with CA
	suite.Nil(err)
	_, err = suite.RegistryClientWithCA.PullChart(ref)
	suite.Nil(err)

	// TLS registry with cert verification enabled
	ref, err = ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", suite.TLSVerifyClientDockerRegistryHost))
	suite.Nil(err)
	_, err = suite.RegistryClientWithCertKey.PullChart(ref)
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
	suite.NotNil(err, "no error logging out of registry that has no entry")

	err = suite.RegistryClient.Logout(suite.DockerRegistryHost)
	suite.Nil(err, "error logging out of registry")

	err = suite.PlainHTTPRegistryClient.Logout(suite.PlainHTTPDockerRegistryHost)
	suite.Nil(err, "error logging out of plain http registry")

	err = suite.InsecureRegistryClient.Logout(suite.TLSDockerRegistryHost)
	suite.Nil(err, "error logging out of insecure registry")

	// error as logout happened for TLSDockerRegistryHost in last step
	err = suite.RegistryClientWithCA.Logout(suite.TLSDockerRegistryHost)
	suite.NotNil(err, "no error logging out of insecure registry with ca cert")
}

func (suite *RegistryClientTestSuite) Test_8_ManInTheMiddle() {
	ref, err := ParseReference(fmt.Sprintf("%s/testrepo/supposedlysafechart:9.9.9", suite.CompromisedRegistryHost))
	suite.Nil(err)

	// returns content that does not match the expected digest
	_, err = suite.RegistryClient.PullChart(ref)
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
				`{ "schemaVersion": 2, "config": {
    "mediaType": "application/vnd.cncf.helm.config.v1+json",
    "digest": "sha256:a705ee2789ab50a5ba20930f246dbd5cc01ff9712825bb98f57ee8414377f133",
    "size": 181
  },
  "layers": [
    {
      "mediaType": "application/tar+gzip",
      "digest": "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
      "size": 1
    }
  ]
}`))
		} else if r.URL.Path == "/v2/testrepo/supposedlysafechart/blobs/sha256:a705ee2789ab50a5ba20930f246dbd5cc01ff9712825bb98f57ee8414377f133" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte("{\"name\":\"mychart\",\"version\":\"0.1.0\",\"description\":\"A Helm chart for Kubernetes\\n" +
				"an 'application' or a 'library' chart.\",\"apiVersion\":\"v2\",\"appVersion\":\"1.16.0\",\"type\":" +
				"\"application\"}"))
		} else if r.URL.Path == "/v2/testrepo/supposedlysafechart/blobs/sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
			w.Header().Set("Content-Type", "application/tar+gzip")
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
