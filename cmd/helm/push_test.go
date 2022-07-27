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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/action"
	helmRegistry "helm.sh/helm/v3/pkg/registry"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
)

func TestPushFileCompletion(t *testing.T) {
	checkFileCompletion(t, "push", true)
	checkFileCompletion(t, "push package.tgz", false)
	checkFileCompletion(t, "push package.tgz oci://localhost:5000", false)
}

const (
	testWorkspaceDir = "helm-registry-test"
)

type HelmPushTestSuite struct {
	suite.Suite
	Out                     io.Writer
	DockerRegistryHost      string
	CompromisedRegistryHost string
	WorkspaceDir            string
	RegistryClient          *helmRegistry.Client
	PushClient              *action.Push
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// filter loopback ip
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", errors.New("can not find the client ip address")

}

func TestGetLocalIP(t *testing.T) {
	ip, err := getLocalIP()
	if err != nil {
		t.Errorf("get local ip error %+v", err)
	}
	t.Log(ip)
}

func (suite *HelmPushTestSuite) SetupSuite() {
	suite.WorkspaceDir = testWorkspaceDir
	os.RemoveAll(suite.WorkspaceDir)
	os.Mkdir(suite.WorkspaceDir, 0700)

	var out bytes.Buffer
	suite.Out = &out

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test registry")

	ip, err := getLocalIP()
	suite.Nil(err, "get host ip error")
	suite.DockerRegistryHost = fmt.Sprintf("%s:%d", ip, port)
	config.HTTP.Addr = fmt.Sprintf("%s:%d", ip, port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	suite.Nil(err, "no error creating test registry")

	suite.CompromisedRegistryHost = fmt.Sprintf("%s:%d", ip, port)

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *HelmPushTestSuite) TestSecretPush() {
	// init test client
	var err error
	suite.RegistryClient, err = helmRegistry.NewClient(
		helmRegistry.ClientOptDebug(true),
		helmRegistry.ClientOptEnableCache(true),
		helmRegistry.ClientOptWriter(suite.Out),
	)
	suite.Nil(err, "no error creating registry client")

	// init push client
	cfg := &action.Configuration{
		RegistryClient: suite.RegistryClient,
	}
	suite.PushClient = action.NewPushWithOpts(action.WithPushConfig(cfg))
	ref := fmt.Sprintf("oci://%s/testrepo", suite.DockerRegistryHost)
	// Load a test chart
	_, err = suite.PushClient.Run("./testdata/testcharts/examplechart-0.1.0.tgz", ref)
	suite.ErrorContains(err, "http: server gave HTTP response to HTTPS client")
}

func (suite *HelmPushTestSuite) TestInsecretPush() {
	// init test client
	var err error
	suite.RegistryClient, err = helmRegistry.NewClient(
		helmRegistry.ClientOptDebug(true),
		helmRegistry.ClientOptEnableCache(true),
		helmRegistry.ClientOptWriter(suite.Out),
		helmRegistry.ClientPlainHTTP(),
	)
	suite.Nil(err, "no error creating registry client")

	// init push client
	cfg := &action.Configuration{
		RegistryClient: suite.RegistryClient,
	}
	suite.PushClient = action.NewPushWithOpts(action.WithPushConfig(cfg))
	ref := fmt.Sprintf("oci://%s/testrepo", suite.DockerRegistryHost)
	// Load a test chart
	_, err = suite.PushClient.Run("./testdata/testcharts/examplechart-0.1.0.tgz", ref)
	suite.Nil(err, "no error loading test chart")

}

func TestHelmPushTestSuite(t *testing.T) {
	suite.Run(t, new(HelmPushTestSuite))
}
