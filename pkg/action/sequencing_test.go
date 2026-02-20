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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestSplitManifestsBySubchart(t *testing.T) {
	manifest := `---
# Source: myapp/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp-svc
---
# Source: myapp/charts/redis/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
---
# Source: myapp/charts/redis/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: redis-svc
---
# Source: myapp/charts/nginx/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx`

	result := SplitManifestsBySubchart(manifest, "myapp")

	assert.Len(t, result, 3)
	assert.Contains(t, result, "myapp")
	assert.Contains(t, result, "redis")
	assert.Contains(t, result, "nginx")

	// Parent chart has 1 resource
	assert.Contains(t, result["myapp"], "myapp-svc")

	// Redis has 2 resources
	assert.Contains(t, result["redis"], "redis")
	assert.Contains(t, result["redis"], "redis-svc")

	// Nginx has 1 resource
	assert.Contains(t, result["nginx"], "nginx")
}

func TestSplitManifestsBySubchartNested(t *testing.T) {
	manifest := `---
# Source: parent/charts/redis/charts/sentinel/templates/deploy.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sentinel`

	result := SplitManifestsBySubchart(manifest, "parent")

	// Nested subchart sentinel under redis → immediate child is "redis"
	assert.Contains(t, result, "redis")
	assert.Contains(t, result["redis"], "sentinel")
}

func TestSplitManifestsBySubchartEmpty(t *testing.T) {
	result := SplitManifestsBySubchart("", "myapp")
	assert.Empty(t, result)
}

func TestSplitManifestsBySubchartNoSource(t *testing.T) {
	manifest := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: orphan`

	result := SplitManifestsBySubchart(manifest, "myapp")

	// No Source comment → attributed to parent
	assert.Contains(t, result, "myapp")
	assert.Contains(t, result["myapp"], "orphan")
}

func TestSplitManifestDocs(t *testing.T) {
	manifest := "---\napiVersion: v1\nkind: ConfigMap\n---\napiVersion: apps/v1\nkind: Deployment\n---\n"
	docs := SplitManifestDocs(manifest)
	assert.Len(t, docs, 2)
}

func TestBuildResourceGroupBatchesNone(t *testing.T) {
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"
	result, err := BuildResourceGroupBatches(manifest)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestBuildResourceGroupBatchesLinear(t *testing.T) {
	manifest := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: db-config
  annotations:
    helm.sh/resource-group: database
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deploy
  annotations:
    helm.sh/resource-group: application
    helm.sh/depends-on/resource-groups: '["database"]'`

	result, err := BuildResourceGroupBatches(manifest)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, [][]string{{"database"}, {"application"}}, result.Batches)
	assert.Contains(t, result.GroupedManifests, "database")
	assert.Contains(t, result.GroupedManifests, "application")
	assert.Empty(t, result.UnsequencedManifest)
}

func TestBuildResourceGroupBatchesMixed(t *testing.T) {
	manifest := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: db-config
  annotations:
    helm.sh/resource-group: database
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: orphan`

	result, err := BuildResourceGroupBatches(manifest)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, [][]string{{"database"}}, result.Batches)
	assert.NotEmpty(t, result.UnsequencedManifest)
	assert.Contains(t, result.UnsequencedManifest, "orphan")
}

func TestBuildResourceGroupBatchesWarnings(t *testing.T) {
	manifest := `---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    helm.sh/resource-group: app
    helm.sh/depends-on/resource-groups: '["nonexistent"]'`

	result, err := BuildResourceGroupBatches(manifest)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "nonexistent")
}

func TestReorderManifestForTemplateNoSequencing(t *testing.T) {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "myapp",
		},
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"
	result := ReorderManifestForTemplate(manifest, chrt)
	assert.Equal(t, manifest, result, "unchanged when no sequencing")
}

func TestReorderManifestForTemplateWithSequencing(t *testing.T) {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "parent",
			Dependencies: []*chart.Dependency{
				{Name: "redis"},
				{Name: "app", DependsOn: []string{"redis"}},
			},
			Annotations: map[string]string{
				chartutil.AnnotationDependsOnSubcharts: `["app"]`,
			},
		},
	}

	manifest := `---
# Source: parent/charts/redis/templates/deploy.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
---
# Source: parent/charts/app/templates/deploy.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
---
# Source: parent/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: parent-svc`

	result := ReorderManifestForTemplate(manifest, chrt)
	// Redis should appear before app, and parent should be last
	redisIdx := strings.Index(result, "redis")
	appIdx := strings.Index(result, "name: app")
	parentIdx := strings.Index(result, "parent-svc")

	assert.True(t, redisIdx < appIdx, "redis should come before app")
	assert.True(t, appIdx < parentIdx, "app should come before parent")
}

func TestBuildInstallBatchesNoSequencing(t *testing.T) {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "myapp",
			Dependencies: []*chart.Dependency{
				{Name: "redis"},
				{Name: "nginx"},
			},
		},
	}

	batches, err := BuildInstallBatches(chrt)
	require.NoError(t, err)
	assert.Nil(t, batches, "no batches when no sequencing declared")
}

func TestBuildInstallBatchesWithDependsOn(t *testing.T) {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "foo",
			Annotations: map[string]string{
				chartutil.AnnotationDependsOnSubcharts: `["bar", "rabbitmq"]`,
			},
			Dependencies: []*chart.Dependency{
				{Name: "nginx"},
				{Name: "rabbitmq"},
				{
					Name:      "bar",
					DependsOn: []string{"nginx", "rabbitmq"},
				},
			},
		},
	}

	batches, err := BuildInstallBatches(chrt)
	require.NoError(t, err)
	require.Len(t, batches, 3)
	assert.Equal(t, []string{"nginx", "rabbitmq"}, batches[0])
	assert.Equal(t, []string{"bar"}, batches[1])
	assert.Equal(t, []string{"foo"}, batches[2])
}

func TestBuildInstallBatchesCircularDep(t *testing.T) {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "parent",
			Dependencies: []*chart.Dependency{
				{Name: "A", DependsOn: []string{"B"}},
				{Name: "B", DependsOn: []string{"A"}},
			},
		},
	}

	_, err := BuildInstallBatches(chrt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}
