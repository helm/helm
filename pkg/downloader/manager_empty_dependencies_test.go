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

package downloader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestUpdateAfterRemovingAllDependencies(t *testing.T) {
	for _, apiVersion := range []string{"v1", "v2"} {
		for _, removed := range []struct {
			name string
			yaml string
		}{
			{name: "absent"},
			{name: "null", yaml: "dependencies: null\n"},
			{name: "empty list", yaml: "dependencies: []\n"},
		} {
			t.Run(apiVersion+"/"+removed.name, func(t *testing.T) {
				root := t.TempDir()
				dep := &chart.Chart{Metadata: &chart.Metadata{Name: "dep", Version: "1.0.0", APIVersion: "v2"}}
				require.NoError(t, chartutil.SaveDir(dep, root))
				chartDir := filepath.Join(root, "parent")
				require.NoError(t, os.Mkdir(chartDir, 0o755))
				chartFile := filepath.Join(chartDir, "Chart.yaml")
				metadata := fmt.Sprintf("apiVersion: %s\nname: parent\nversion: 1.0.0\n", apiVersion)
				dependencies := "dependencies:\n  - name: dep\n    version: 1.0.0\n    repository: file://../dep\n"
				dependencyFile := chartFile
				lockFile := filepath.Join(chartDir, "Chart.lock")
				if apiVersion == "v1" {
					require.NoError(t, os.WriteFile(chartFile, []byte(metadata), 0o644))
					dependencyFile = filepath.Join(chartDir, "requirements.yaml")
					lockFile = filepath.Join(chartDir, "requirements.lock")
				} else {
					dependencies = metadata + dependencies
				}
				require.NoError(t, os.WriteFile(dependencyFile, []byte(dependencies), 0o644))
				manager := &Manager{
					ChartPath: chartDir, Out: io.Discard, SkipUpdate: true,
					RepositoryConfig: filepath.Join(root, "repositories.yaml"),
					RepositoryCache:  filepath.Join(root, "cache"),
				}
				require.NoError(t, manager.Update())
				require.FileExists(t, lockFile)
				archive := filepath.Join(chartDir, "charts", "dep-1.0.0.tgz")
				require.FileExists(t, archive)

				switch {
				case apiVersion == "v2":
					require.NoError(t, os.WriteFile(chartFile, []byte(metadata+removed.yaml), 0o644))
				case removed.yaml == "":
					require.NoError(t, os.Remove(dependencyFile))
				default:
					require.NoError(t, os.WriteFile(dependencyFile, []byte(removed.yaml), 0o644))
				}
				require.NoError(t, manager.Update())
				updated, err := loader.LoadDir(chartDir)
				require.NoError(t, err)
				require.NotNil(t, updated.Lock)
				assert.Empty(t, updated.Lock.Dependencies)
				assert.NoFileExists(t, archive)
				require.NoError(t, manager.Build(), "build must accept the updated lock file")

				before, err := os.ReadFile(lockFile)
				require.NoError(t, err)
				require.NoError(t, manager.Update())
				after, err := os.ReadFile(lockFile)
				require.NoError(t, err)
				assert.Equal(t, before, after, "repeated updates must leave the lock file unchanged")
			})
		}
	}
}

func TestUpdateWithoutManagedDependencies(t *testing.T) {
	for _, apiVersion := range []string{"v1", "v2"} {
		t.Run(apiVersion, func(t *testing.T) {
			root := t.TempDir()
			parent := &chart.Chart{Metadata: &chart.Metadata{Name: "parent", Version: "1.0.0", APIVersion: apiVersion}}
			require.NoError(t, chartutil.SaveDir(parent, root))
			chartDir := filepath.Join(root, "parent")
			dep := &chart.Chart{Metadata: &chart.Metadata{Name: "manual", Version: "1.0.0", APIVersion: "v2"}}
			archive, err := chartutil.Save(dep, filepath.Join(chartDir, "charts"))
			require.NoError(t, err)
			before, err := os.ReadFile(archive)
			require.NoError(t, err)
			manager := &Manager{ChartPath: chartDir, Out: io.Discard, RepositoryConfig: filepath.Join(root, "repositories.yaml")}
			require.NoError(t, manager.Update())
			require.NoError(t, manager.Build())
			after, err := os.ReadFile(archive)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.NoFileExists(t, filepath.Join(chartDir, "Chart.lock"))
			assert.NoFileExists(t, filepath.Join(chartDir, "requirements.lock"))
		})
	}
}
