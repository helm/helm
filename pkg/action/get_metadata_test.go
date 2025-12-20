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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ci "helm.sh/helm/v4/pkg/chart"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewGetMetadata(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.cfg)
	assert.Equal(t, 0, client.Version)
}

func TestGetMetadata_Run_BasicMetadata(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "test-chart",
				Version:    "1.0.0",
				AppVersion: "v1.2.3",
			},
		},
		Version:   1,
		Namespace: "default",
	}

	err := cfg.Releases.Create(rel)
	require.NoError(t, err)

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "v1.2.3", result.AppVersion)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, 1, result.Revision)
	assert.Equal(t, "deployed", result.Status)
	assert.Equal(t, deployedTime.Format(time.RFC3339), result.DeployedAt)
	assert.Empty(t, result.Dependencies)
	assert.Empty(t, result.Annotations)
}

func TestGetMetadata_Run_WithDependencies(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	dependencies := []*chart.Dependency{
		{
			Name:       "mysql",
			Version:    "8.0.25",
			Repository: "https://charts.bitnami.com/bitnami",
		},
		{
			Name:       "redis",
			Version:    "6.2.4",
			Repository: "https://charts.bitnami.com/bitnami",
		},
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:         "test-chart",
				Version:      "1.0.0",
				AppVersion:   "v1.2.3",
				Dependencies: dependencies,
			},
		},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	dep0, err := ci.NewDependencyAccessor(result.Dependencies[0])
	require.NoError(t, err)
	dep1, err := ci.NewDependencyAccessor(result.Dependencies[1])
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, convertDeps(dependencies), result.Dependencies)
	assert.Len(t, result.Dependencies, 2)
	assert.Equal(t, "mysql", dep0.Name())
	assert.Equal(t, "redis", dep1.Name())
}

func TestGetMetadata_Run_WithDependenciesAliases(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	dependencies := []*chart.Dependency{
		{
			Name:       "mysql",
			Version:    "8.0.25",
			Repository: "https://charts.bitnami.com/bitnami",
			Alias:      "database",
		},
		{
			Name:       "redis",
			Version:    "6.2.4",
			Repository: "https://charts.bitnami.com/bitnami",
			Alias:      "cache",
		},
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:         "test-chart",
				Version:      "1.0.0",
				AppVersion:   "v1.2.3",
				Dependencies: dependencies,
			},
		},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	dep0, err := ci.NewDependencyAccessor(result.Dependencies[0])
	require.NoError(t, err)
	dep1, err := ci.NewDependencyAccessor(result.Dependencies[1])
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, convertDeps(dependencies), result.Dependencies)
	assert.Len(t, result.Dependencies, 2)
	assert.Equal(t, "mysql", dep0.Name())
	assert.Equal(t, "database", dep0.Alias())
	assert.Equal(t, "redis", dep1.Name())
	assert.Equal(t, "cache", dep1.Alias())
}

func TestGetMetadata_Run_WithMixedDependencies(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	dependencies := []*chart.Dependency{
		{
			Name:       "mysql",
			Version:    "8.0.25",
			Repository: "https://charts.bitnami.com/bitnami",
			Alias:      "database",
		},
		{
			Name:       "nginx",
			Version:    "1.20.0",
			Repository: "https://charts.bitnami.com/bitnami",
		},
		{
			Name:       "redis",
			Version:    "6.2.4",
			Repository: "https://charts.bitnami.com/bitnami",
			Alias:      "cache",
		},
		{
			Name:       "postgresql",
			Version:    "11.0.0",
			Repository: "https://charts.bitnami.com/bitnami",
		},
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:         "test-chart",
				Version:      "1.0.0",
				AppVersion:   "v1.2.3",
				Dependencies: dependencies,
			},
		},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	dep0, err := ci.NewDependencyAccessor(result.Dependencies[0])
	require.NoError(t, err)
	dep1, err := ci.NewDependencyAccessor(result.Dependencies[1])
	require.NoError(t, err)
	dep2, err := ci.NewDependencyAccessor(result.Dependencies[2])
	require.NoError(t, err)
	dep3, err := ci.NewDependencyAccessor(result.Dependencies[3])
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, convertDeps(dependencies), result.Dependencies)
	assert.Len(t, result.Dependencies, 4)

	// Verify dependencies with aliases
	assert.Equal(t, "mysql", dep0.Name())
	assert.Equal(t, "database", dep0.Alias())
	assert.Equal(t, "redis", dep2.Name())
	assert.Equal(t, "cache", dep2.Alias())

	// Verify dependencies without aliases
	assert.Equal(t, "nginx", dep1.Name())
	assert.Equal(t, "", dep1.Alias())
	assert.Equal(t, "postgresql", dep3.Name())
	assert.Equal(t, "", dep3.Alias())
}

func TestGetMetadata_Run_WithAnnotations(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	annotations := map[string]string{
		"helm.sh/hook":        "pre-install",
		"helm.sh/hook-weight": "5",
		"custom.annotation":   "test-value",
	}

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:        "test-chart",
				Version:     "1.0.0",
				AppVersion:  "v1.2.3",
				Annotations: annotations,
			},
		},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, annotations, result.Annotations)
	assert.Equal(t, "pre-install", result.Annotations["helm.sh/hook"])
	assert.Equal(t, "5", result.Annotations["helm.sh/hook-weight"])
	assert.Equal(t, "test-value", result.Annotations["custom.annotation"])
}

func TestGetMetadata_Run_SpecificVersion(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)
	client.Version = 2

	releaseName := "test-release"
	deployedTime := time.Now()

	rel1 := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusSuperseded,
			LastDeployed: deployedTime.Add(-time.Hour),
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "test-chart",
				Version:    "1.0.0",
				AppVersion: "v1.0.0",
			},
		},
		Version:   1,
		Namespace: "default",
	}

	rel2 := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "test-chart",
				Version:    "1.1.0",
				AppVersion: "v1.1.0",
			},
		},
		Version:   2,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel1))
	require.NoError(t, cfg.Releases.Create(rel2))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, "test-chart", result.Chart)
	assert.Equal(t, "1.1.0", result.Version)
	assert.Equal(t, "v1.1.0", result.AppVersion)
	assert.Equal(t, 2, result.Revision)
	assert.Equal(t, "deployed", result.Status)
}

func TestGetMetadata_Run_DifferentStatuses(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	testCases := []struct {
		name     string
		status   common.Status
		expected string
	}{
		{"deployed", common.StatusDeployed, "deployed"},
		{"failed", common.StatusFailed, "failed"},
		{"uninstalled", common.StatusUninstalled, "uninstalled"},
		{"pending-install", common.StatusPendingInstall, "pending-install"},
		{"pending-upgrade", common.StatusPendingUpgrade, "pending-upgrade"},
		{"pending-rollback", common.StatusPendingRollback, "pending-rollback"},
		{"superseded", common.StatusSuperseded, "superseded"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			releaseName := "test-release-" + tc.name
			deployedTime := time.Now()

			rel := &release.Release{
				Name: releaseName,
				Info: &release.Info{
					Status:       tc.status,
					LastDeployed: deployedTime,
				},
				Chart: &chart.Chart{
					Metadata: &chart.Metadata{
						Name:       "test-chart",
						Version:    "1.0.0",
						AppVersion: "v1.0.0",
					},
				},
				Version:   1,
				Namespace: "default",
			}

			require.NoError(t, cfg.Releases.Create(rel))

			result, err := client.Run(releaseName)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, result.Status)
		})
	}
}

func TestGetMetadata_Run_UnreachableKubeClient(t *testing.T) {
	cfg := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	cfg.KubeClient = &failingKubeClient

	client := NewGetMetadata(cfg)

	_, err := client.Run("test-release")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestGetMetadata_Run_ReleaseNotFound(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	_, err := client.Run("non-existent-release")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetMetadata_Run_EmptyAppVersion(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewGetMetadata(cfg)

	releaseName := "test-release"
	deployedTime := time.Now()

	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status:       common.StatusDeployed,
			LastDeployed: deployedTime,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "test-chart",
				Version:    "1.0.0",
				AppVersion: "", // Empty app version
			},
		},
		Version:   1,
		Namespace: "default",
	}

	require.NoError(t, cfg.Releases.Create(rel))

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Equal(t, "", result.AppVersion)
}

func TestMetadata_FormattedDepNames(t *testing.T) {
	testCases := []struct {
		name         string
		dependencies []*chart.Dependency
		expected     string
	}{
		{
			name:         "no dependencies",
			dependencies: []*chart.Dependency{},
			expected:     "",
		},
		{
			name: "single dependency",
			dependencies: []*chart.Dependency{
				{Name: "mysql"},
			},
			expected: "mysql",
		},
		{
			name: "multiple dependencies sorted",
			dependencies: []*chart.Dependency{
				{Name: "redis"},
				{Name: "mysql"},
				{Name: "nginx"},
			},
			expected: "mysql,nginx,redis",
		},
		{
			name: "already sorted dependencies",
			dependencies: []*chart.Dependency{
				{Name: "apache"},
				{Name: "mysql"},
				{Name: "zookeeper"},
			},
			expected: "apache,mysql,zookeeper",
		},
		{
			name: "duplicate names",
			dependencies: []*chart.Dependency{
				{Name: "mysql"},
				{Name: "redis"},
				{Name: "mysql"},
			},
			expected: "mysql,mysql,redis",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := convertDeps(tc.dependencies)
			metadata := &Metadata{
				Dependencies: deps,
			}

			result := metadata.FormattedDepNames()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func convertDeps(deps []*chart.Dependency) []ci.Dependency {
	var newDeps = make([]ci.Dependency, len(deps))
	for i, c := range deps {
		newDeps[i] = c
	}
	return newDeps
}

func TestMetadata_FormattedDepNames_WithComplexDependencies(t *testing.T) {
	dependencies := []*chart.Dependency{
		{
			Name:       "zookeeper",
			Version:    "10.0.0",
			Repository: "https://charts.bitnami.com/bitnami",
			Condition:  "zookeeper.enabled",
		},
		{
			Name:       "apache",
			Version:    "9.0.0",
			Repository: "https://charts.bitnami.com/bitnami",
		},
		{
			Name:       "mysql",
			Version:    "8.0.25",
			Repository: "https://charts.bitnami.com/bitnami",
			Condition:  "mysql.enabled",
		},
	}

	deps := convertDeps(dependencies)
	metadata := &Metadata{
		Dependencies: deps,
	}

	result := metadata.FormattedDepNames()
	assert.Equal(t, "apache,mysql,zookeeper", result)
}

func TestMetadata_FormattedDepNames_WithAliases(t *testing.T) {
	testCases := []struct {
		name         string
		dependencies []*chart.Dependency
		expected     string
	}{
		{
			name: "dependencies with aliases",
			dependencies: []*chart.Dependency{
				{Name: "mysql", Alias: "database"},
				{Name: "redis", Alias: "cache"},
			},
			expected: "mysql,redis",
		},
		{
			name: "mixed dependencies with and without aliases",
			dependencies: []*chart.Dependency{
				{Name: "mysql", Alias: "database"},
				{Name: "nginx"},
				{Name: "redis", Alias: "cache"},
			},
			expected: "mysql,nginx,redis",
		},
		{
			name: "empty alias should use name",
			dependencies: []*chart.Dependency{
				{Name: "mysql", Alias: ""},
				{Name: "redis", Alias: "cache"},
			},
			expected: "mysql,redis",
		},
		{
			name: "sorted by name not alias",
			dependencies: []*chart.Dependency{
				{Name: "zookeeper", Alias: "a-service"},
				{Name: "apache", Alias: "z-service"},
			},
			expected: "apache,zookeeper",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := convertDeps(tc.dependencies)
			metadata := &Metadata{
				Dependencies: deps,
			}

			result := metadata.FormattedDepNames()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetMetadata_Labels(t *testing.T) {
	rel := releaseStub()
	rel.Info.Status = common.StatusDeployed
	customLabels := map[string]string{"key1": "value1", "key2": "value2"}
	rel.Labels = customLabels

	metaGetter := NewGetMetadata(actionConfigFixture(t))
	err := metaGetter.cfg.Releases.Create(rel)
	assert.NoError(t, err)

	metadata, err := metaGetter.Run(rel.Name)
	assert.NoError(t, err)

	assert.Equal(t, metadata.Name, rel.Name)
	assert.Equal(t, metadata.Labels, customLabels)
}
