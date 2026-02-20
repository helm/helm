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
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestBuildInstallBatchesWithResourceGroups(t *testing.T) {
	// Test that BuildInstallBatches correctly builds subchart batches
	// Resource-group batching is tested separately through BuildResourceGroupBatches
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "myapp",
			Dependencies: []*chart.Dependency{
				{Name: "database", DependsOn: []string{}},
				{Name: "cache"},
				{Name: "api", DependsOn: []string{"database", "cache"}},
			},
			Annotations: map[string]string{
				chartutil.AnnotationDependsOnSubcharts: `["api"]`,
			},
		},
	}

	batches, err := BuildInstallBatches(chrt)
	require.NoError(t, err)
	require.Len(t, batches, 3)
	assert.Equal(t, []string{"cache", "database"}, batches[0])
	assert.Equal(t, []string{"api"}, batches[1])
	assert.Equal(t, []string{"myapp"}, batches[2])
}

func TestSequencingMetadataResourceGroupBatches(t *testing.T) {
	// Verify the release SequencingMetadata can store resource-group batches
	seq := &release.SequencingMetadata{
		Enabled:  true,
		Strategy: "ordered",
		Batches:  [][]string{{"db"}, {"app"}, {"parent"}},
		ResourceGroupBatches: map[string][][]string{
			"app": {{"database"}, {"application"}},
		},
	}

	assert.True(t, seq.Enabled)
	assert.Len(t, seq.Batches, 3)
	assert.Contains(t, seq.ResourceGroupBatches, "app")
	assert.Len(t, seq.ResourceGroupBatches["app"], 2)
}

func TestSplitManifestsBySubchartConsistency(t *testing.T) {
	// Ensure subchart splitting works consistently for ordered operations
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
# Source: myapp/charts/nginx/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx`

	result := SplitManifestsBySubchart(manifest, "myapp")

	// Should produce 3 buckets
	assert.Len(t, result, 3)
	assert.Contains(t, result, "myapp")
	assert.Contains(t, result, "redis")
	assert.Contains(t, result, "nginx")

	// Parent chart
	assert.Contains(t, result["myapp"], "myapp-svc")
	// Subcharts
	assert.Contains(t, result["redis"], "redis")
	assert.Contains(t, result["nginx"], "nginx")
}
