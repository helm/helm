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
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func TestValidRegistryUrlWithImageTag(t *testing.T) {
	os.RemoveAll(testCacheRootDir)
	os.Mkdir(testCacheRootDir, 0700)

	var out bytes.Buffer
	credentialsFile := filepath.Join(testCacheRootDir, CredentialsFileBasename)

	client, err := auth.NewClient(credentialsFile)
	assert.Nil(t, err, "no error creating auth client")

	resolver, err := client.Resolver(context.Background(), http.DefaultClient, false)
	assert.Nil(t, err, "no error creating resolver")

	// create cache
	cache, err := NewCache(
		CacheOptDebug(true),
		CacheOptWriter(&out),
		CacheOptRoot(filepath.Join(testCacheRootDir, CacheRootDir)),
	)
	assert.Nil(t, err, "no error creating cache")

	// init test client
	registryClient, err := NewClient(
		ClientOptDebug(true),
		ClientOptWriter(&out),
		ClientOptAuthorizer(&Authorizer{
			Client: client,
		}),
		ClientOptResolver(&Resolver{
			Resolver: resolver,
		}),
		ClientOptCache(cache),
	)
	assert.Nil(t, err, "no error creating registry client")

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	assert.Nil(t, err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(testCacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	assert.Nil(t, err, "no error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	assert.Nil(t, err, "failed to find free port for test registry")
	dockerRegistryHost := fmt.Sprintf("localhost:%d", port)
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
	assert.Nil(t, err, "failed to create test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
	registryClient.Login(dockerRegistryHost, testUsername, testPassword, false)

	ref, _ := ParseReference(fmt.Sprintf("%s/testrepo/testchart:1.2.3", dockerRegistryHost))

	ch := &chart.Chart{}
	ch.Metadata = &chart.Metadata{
		APIVersion: "v1",
		Name:       "testchart",
		Version:    "1.2.3",
	}

	err = registryClient.SaveChart(ch, ref)
	assert.NoError(t, err)
	err = registryClient.PushChart(ref)
	assert.NoError(t, err)

	g := NewRegistryGetter(registryClient)
	res, err := g.Get(fmt.Sprintf("oci://%s/testrepo/testchart:1.2.3", dockerRegistryHost))
	assert.NoError(t, err, "failed to retrieve chart")

	downloadedChart, err := loader.LoadArchive(res)
	assert.NoError(t, err, "failed to load archive")
	assert.Equal(t, "testchart", downloadedChart.Name())
	assert.Equal(t, "1.2.3", downloadedChart.Metadata.Version)

	registryClient.Logout(dockerRegistryHost)
	os.RemoveAll(testCacheRootDir)
}
