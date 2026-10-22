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

package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	rspb "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart/common"
	rcommon "helm.sh/helm/v4/pkg/release/common"
)

// chartFixture builds a chart with a dependency, raw files and nested values so
// the copy can be checked against the parts a JSON round-trip would drop.
func chartFixture() *chart.Chart {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:        "parent",
			Annotations: map[string]string{"example.com/team": "helm"},
			Maintainers: []*chart.Maintainer{{Name: "maintainer"}},
		},
		Raw:       []*common.File{{Name: "Chart.yaml", Data: []byte("name: parent")}},
		Templates: []*common.File{{Name: "templates/cm.yaml", Data: []byte("kind: ConfigMap")}},
		Values:    map[string]any{"nested": map[string]any{"replicas": 1}},
		Schema:    []byte("schema-bytes"),
	}
	ch.AddDependency(&chart.Chart{Metadata: &chart.Metadata{Name: "child"}})
	return ch
}

func releaseFixture() *rspb.Release {
	rls := releaseStub("copy-me", 1, "default", rcommon.StatusDeployed)
	rls.Chart = chartFixture()
	rls.Config = map[string]any{"nested": map[string]any{"enabled": true}}
	rls.Hooks = []*rspb.Hook{{
		Name:   "pre-install",
		Events: []rspb.HookEvent{rspb.HookPreInstall},
	}}
	return rls
}

func TestCopyReleaseIsIndependent(t *testing.T) {
	rls := releaseFixture()
	copied := copyRelease(rls)

	require.NotSame(t, rls, copied)
	assert.Equal(t, rls, copied, "a copy should hold the same data as the original")

	copied.Info.Status = rcommon.StatusFailed
	copied.Labels["key1"] = "changed"
	copied.Config["nested"].(map[string]any)["enabled"] = false
	copied.Chart.Metadata.Name = "renamed"
	copied.Chart.Metadata.Annotations["example.com/team"] = "changed"
	copied.Chart.Metadata.Maintainers[0].Name = "changed"
	copied.Chart.Values["nested"].(map[string]any)["replicas"] = 2
	copied.Chart.Raw[0].Data[0] = 'X'
	copied.Chart.Schema[0] = 'X'
	copied.Chart.Dependencies()[0].Metadata.Name = "changed"
	copied.Hooks[0].Events[0] = rspb.HookPostInstall

	assert.Equal(t, rcommon.StatusDeployed, rls.Info.Status)
	assert.Equal(t, "val1", rls.Labels["key1"])
	assert.Equal(t, true, rls.Config["nested"].(map[string]any)["enabled"])
	assert.Equal(t, "parent", rls.Chart.Metadata.Name)
	assert.Equal(t, "helm", rls.Chart.Metadata.Annotations["example.com/team"])
	assert.Equal(t, "maintainer", rls.Chart.Metadata.Maintainers[0].Name)
	assert.Equal(t, 1, rls.Chart.Values["nested"].(map[string]any)["replicas"])
	assert.Equal(t, []byte("name: parent"), rls.Chart.Raw[0].Data)
	assert.Equal(t, []byte("schema-bytes"), rls.Chart.Schema)
	assert.Equal(t, "child", rls.Chart.Dependencies()[0].Metadata.Name)
	assert.Equal(t, rspb.HookPreInstall, rls.Hooks[0].Events[0])
}

// The chart tree is rebuilt through AddDependency, so the copied dependency has
// to point at the copied parent rather than the original one.
func TestCopyReleaseRebuildsChartTree(t *testing.T) {
	copied := copyRelease(releaseFixture())

	require.Len(t, copied.Chart.Dependencies(), 1)
	dep := copied.Chart.Dependencies()[0]
	assert.Same(t, copied.Chart, dep.Parent())
	assert.True(t, copied.Chart.IsRoot())
	assert.Equal(t, "parent.child", dep.ChartPath())
}

func TestCopyReleaseKeepsNilFields(t *testing.T) {
	copied := copyRelease(&rspb.Release{Name: "sparse"})

	assert.Equal(t, "sparse", copied.Name)
	assert.Nil(t, copied.Info)
	assert.Nil(t, copied.Chart)
	assert.Nil(t, copied.Config)
	assert.Nil(t, copied.Hooks)
	assert.Nil(t, copied.Labels)
	assert.Nil(t, copyRelease(nil))
}
