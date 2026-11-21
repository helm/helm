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

package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestPlanTemplateManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan *Plan
		want string
	}{
		{
			name: "one group with two manifests",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "mychart",
						Kind:      BatchKindGroups,
						Groups: []Group{
							{
								Name: "database",
								Manifests: []releaseutil.Manifest{
									{Name: "mychart/templates/db.yaml", Content: "apiVersion: v1\nkind: Service"},
									{Name: "mychart/templates/statefulset.yaml", Content: "apiVersion: apps/v1\nkind: StatefulSet"},
								},
							},
						},
					},
				},
			},
			want: `---
## START resource-group: mychart database
# Source: mychart/templates/db.yaml
apiVersion: v1
kind: Service
---
# Source: mychart/templates/statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
## END resource-group: mychart database
`,
		},
		{
			name: "two groups in one batch",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "mychart",
						Kind:      BatchKindGroups,
						Groups: []Group{
							{
								Name: "database",
								Manifests: []releaseutil.Manifest{
									{Name: "mychart/templates/db.yaml", Content: "apiVersion: v1\nkind: Service"},
								},
							},
							{
								Name: "app",
								Manifests: []releaseutil.Manifest{
									{Name: "mychart/templates/deployment.yaml", Content: "apiVersion: apps/v1\nkind: Deployment"},
								},
							},
						},
					},
				},
			},
			want: `---
## START resource-group: mychart database
# Source: mychart/templates/db.yaml
apiVersion: v1
kind: Service
## END resource-group: mychart database
---
## START resource-group: mychart app
# Source: mychart/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
## END resource-group: mychart app
`,
		},
		{
			name: "groups in a subchart",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "foo/charts/bar",
						Kind:      BatchKindGroups,
						Groups: []Group{
							{
								Name: "redis",
								Manifests: []releaseutil.Manifest{
									{Name: "foo/charts/bar/templates/service.yaml", Content: "apiVersion: v1\nkind: Service"},
								},
							},
						},
					},
				},
			},
			want: `---
## START resource-group: foo/bar redis
# Source: foo/charts/bar/templates/service.yaml
apiVersion: v1
kind: Service
## END resource-group: foo/bar redis
`,
		},
		{
			name: "unsequenced manifests after groups",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "chart",
						Kind:      BatchKindGroups,
						Groups: []Group{
							{
								Name: "database",
								Manifests: []releaseutil.Manifest{
									{Name: "chart/templates/db.yaml", Content: "apiVersion: v1\nkind: Service"},
								},
							},
						},
					},
					{
						ChartPath: "chart",
						Kind:      BatchKindUnsequenced,
						Manifests: []releaseutil.Manifest{
							{Name: "chart/templates/misc.yaml", Content: "apiVersion: v1\nkind: ConfigMap"},
							{Name: "chart/templates/other.yaml", Content: "apiVersion: v1\nkind: Secret"},
						},
					},
				},
			},
			want: `---
## START resource-group: chart database
# Source: chart/templates/db.yaml
apiVersion: v1
kind: Service
## END resource-group: chart database
---
# Source: chart/templates/misc.yaml
apiVersion: v1
kind: ConfigMap
---
# Source: chart/templates/other.yaml
apiVersion: v1
kind: Secret
`,
		},
		{
			name: "only unsequenced manifests",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "simple",
						Kind:      BatchKindUnsequenced,
						Manifests: []releaseutil.Manifest{
							{Name: "simple/templates/service.yaml", Content: "apiVersion: v1\nkind: Service"},
						},
					},
				},
			},
			want: `---
# Source: simple/templates/service.yaml
apiVersion: v1
kind: Service
`,
		},
		{
			name: "nil plan",
			plan: nil,
			want: "",
		},
		{
			name: "empty plan",
			plan: &Plan{Batches: []Batch{}},
			want: "",
		},
		{
			name: "batch with empty groups",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "empty",
						Kind:      BatchKindGroups,
						Groups:    []Group{},
					},
				},
			},
			want: "",
		},
		{
			name: "group with no manifests",
			plan: &Plan{
				Batches: []Batch{
					{
						ChartPath: "test",
						Kind:      BatchKindGroups,
						Groups: []Group{
							{
								Name:      "empty",
								Manifests: []releaseutil.Manifest{},
							},
						},
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.plan.TemplateManifest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateManifestParsable(t *testing.T) {
	t.Parallel()

	plan := &Plan{
		Batches: []Batch{
			{
				ChartPath: "mychart",
				Kind:      BatchKindGroups,
				Groups: []Group{
					{
						Name: "database",
						Manifests: []releaseutil.Manifest{
							{Name: "mychart/templates/db.yaml", Content: "apiVersion: v1\nkind: Service"},
							{Name: "mychart/templates/statefulset.yaml", Content: "apiVersion: apps/v1\nkind: StatefulSet"},
						},
					},
					{
						Name: "app",
						Manifests: []releaseutil.Manifest{
							{Name: "mychart/templates/deployment.yaml", Content: "apiVersion: apps/v1\nkind: Deployment"},
						},
					},
				},
			},
			{
				ChartPath: "mychart",
				Kind:      BatchKindUnsequenced,
				Manifests: []releaseutil.Manifest{
					{Name: "mychart/templates/misc.yaml", Content: "apiVersion: v1\nkind: ConfigMap"},
				},
			},
		},
	}

	output := plan.TemplateManifest()
	manifests, err := ParseStoredManifests(output)
	require.NoError(t, err, "template output must be parsable")

	// Count expected manifests: 2 (database) + 1 (app) + 1 (unsequenced) = 4
	assert.Len(t, manifests, 4, "parsed manifest count must match original")
}
