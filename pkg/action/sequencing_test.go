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
