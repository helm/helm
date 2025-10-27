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

package action

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewGetValues(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.cfg)
	assert.Equal(t, 0, client.Version)
	assert.Equal(t, false, client.AllValues)
}

func TestGetValues_Run_UserConfigOnly(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)

	releaseName := "test-release"
	userConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		},
		"app": map[string]interface{}{
			"name":     "my-app",
			"replicas": 3,
		},
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status: common.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "test-chart",
				Version: "1.0.0",
			},
			Values: map[string]interface{}{
				"defaultKey": "defaultValue",
				"app": map[string]interface{}{
					"name":    "default-app",
					"timeout": 30,
				},
			},
		},
		Config:    userConfig,
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)
	assert.Equal(t, userConfig, result)
}

func TestGetValues_Run_AllValues(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)
	client.AllValues = true

	releaseName := "test-release"
	userConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		},
		"app": map[string]interface{}{
			"name": "my-app",
		},
	}

	chartDefaultValues := map[string]interface{}{
		"defaultKey": "defaultValue",
		"app": map[string]interface{}{
			"name":    "default-app",
			"timeout": 30,
		},
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status: common.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "test-chart",
				Version: "1.0.0",
			},
			Values: chartDefaultValues,
		},
		Config:    userConfig,
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Equal(t, "my-app", result["app"].(map[string]interface{})["name"])
	assert.Equal(t, 30, result["app"].(map[string]interface{})["timeout"])
	assert.Equal(t, "defaultValue", result["defaultKey"])
	assert.Equal(t, "localhost", result["database"].(map[string]interface{})["host"])
	assert.Equal(t, 5432, result["database"].(map[string]interface{})["port"])
}

func TestGetValues_Run_EmptyValues(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)

	releaseName := "test-release"

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status: common.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "test-chart",
				Version: "1.0.0",
			},
		},
		Config:    map[string]interface{}{},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{}, result)
}

func TestGetValues_Run_UnreachableKubeClient(t *testing.T) {
	cfg := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	cfg.KubeClient = &failingKubeClient

	client := NewGetValues(cfg)

	_, err := client.Run("test-release")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestGetValues_Run_ReleaseNotFound(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)

	_, err := client.Run("non-existent-release")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetValues_Run_NilConfig(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetValues(cfg)

	releaseName := "test-release"

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status: common.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "test-chart",
				Version: "1.0.0",
			},
		},
		Config:    nil,
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)
	assert.Nil(t, result)
}
