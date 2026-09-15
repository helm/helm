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

package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func TestSequencing_SubchartCircularDependency(t *testing.T) {
	t.Parallel()

	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Repository: "file://charts/subchart-a", DependsOn: []string{"subchart-b"}},
		{Name: "subchart-b", Version: "0.1.0", Repository: "file://charts/subchart-b", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newChart("subchart-a", nil), newChart("subchart-b", nil))

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, "subchart circular dependency detected")
}

func TestSequencing_AliasedSubchartDependsOnOriginalName(t *testing.T) {
	t.Parallel()

	// Full loader + ProcessDependencies pipeline: an aliased subchart may be
	// referenced by its original chart name (HIP-0025: "names or aliases").
	// Before the fix this reported a spurious "unknown or disabled subchart".
	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Repository: "file://charts/subchart-a", Alias: "aliased-a"},
		{Name: "subchart-b", Version: "0.1.0", Repository: "file://charts/subchart-b", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newChart("subchart-a", nil), newChart("subchart-b", nil))

	messages := runSequencingLint(t, root)
	assert.Empty(t, messages)
}

func TestSequencing_AmbiguousDependsOnReported(t *testing.T) {
	t.Parallel()

	// Ambiguity is now rejected by ProcessDependencies; the lint rule must
	// surface that instead of silently skipping sequencing validation.
	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Repository: "file://charts/subchart-a", Alias: "first"},
		{Name: "subchart-a", Version: "0.1.0", Repository: "file://charts/subchart-a", Alias: "second"},
		{Name: "subchart-b", Version: "0.1.0", Repository: "file://charts/subchart-b", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newChart("subchart-a", nil), newChart("subchart-b", nil))

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, `ambiguous subchart reference "subchart-a"`)
}

func TestSequencing_SubchartAnnotationRequiresHIPListSyntax(t *testing.T) {
	t.Parallel()

	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Repository: "file://charts/subchart-a"},
		{Name: "subchart-b", Version: "0.1.0", Repository: "file://charts/subchart-b"},
	}
	root.Metadata.Annotations = map[string]string{
		chartutil.AnnotationDependsOnSubcharts: `{"subchart-a":["subchart-b"],"subchart-b":["subchart-a"]}`,
	}
	root.SetDependencies(newChart("subchart-a", nil), newChart("subchart-b", nil))

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, "JSON string array")
}

func TestSequencing_SubchartAnnotationOrphanWithNoDependencies(t *testing.T) {
	t.Parallel()

	// A chart with ZERO dependencies can still carry a
	// helm.sh/depends-on/subcharts annotation referencing a subchart that
	// doesn't exist. The lint rule previously early-returned on
	// len(Dependencies)==0 and missed this orphan reference entirely.
	root := newChart("testchart", nil)
	root.Metadata.Annotations = map[string]string{
		chartutil.AnnotationDependsOnSubcharts: `["does-not-exist"]`,
	}

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, "unknown or disabled subchart")
}

func TestSequencing_NestedSubchartCircularDependency(t *testing.T) {
	t.Parallel()

	// The CHILD's Chart.yaml declares a depends-on cycle between its two
	// grandchildren. HEAD's lint only validates the ROOT's subchart DAG, so
	// this chart lints clean but fails fatally at install (bead lkx).
	child := newChart("child", map[string]string{
		"templates/cm.yaml": manifestYAML("ConfigMap", "child-cm", nil),
	})
	child.Metadata.Dependencies = []*chart.Dependency{
		{Name: "grandchild-a", Version: "0.1.0", Repository: "file://charts/grandchild-a", DependsOn: []string{"grandchild-b"}},
		{Name: "grandchild-b", Version: "0.1.0", Repository: "file://charts/grandchild-b", DependsOn: []string{"grandchild-a"}},
	}
	child.SetDependencies(newChart("grandchild-a", nil), newChart("grandchild-b", nil))

	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "child", Version: "0.1.0", Repository: "file://charts/child"},
	}
	root.SetDependencies(child)

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, "subchart circular dependency detected")
	requireMessage(t, messages, support.ErrorSev, "testchart/charts/child")
}

func TestSequencing_NestedSubchartUnknownDependsOnRef(t *testing.T) {
	t.Parallel()

	child := newChart("child", map[string]string{
		"templates/cm.yaml": manifestYAML("ConfigMap", "child-cm", nil),
	})
	child.Metadata.Dependencies = []*chart.Dependency{
		{Name: "grandchild-a", Version: "0.1.0", Repository: "file://charts/grandchild-a", DependsOn: []string{"missing"}},
	}
	child.SetDependencies(newChart("grandchild-a", nil))

	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "child", Version: "0.1.0", Repository: "file://charts/child"},
	}
	root.SetDependencies(child)

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, `depends-on unknown or disabled subchart "missing"`)
	requireMessage(t, messages, support.ErrorSev, "testchart/charts/child")
}

func TestSequencing_NestedResourceGroupCircularDependency(t *testing.T) {
	t.Parallel()

	child := newChart("child", map[string]string{
		"templates/a.yaml": manifestYAML("ConfigMap", "group-a", map[string]string{
			releaseutil.AnnotationResourceGroup:           "a",
			releaseutil.AnnotationDependsOnResourceGroups: `'["b"]'`,
		}),
		"templates/b.yaml": manifestYAML("ConfigMap", "group-b", map[string]string{
			releaseutil.AnnotationResourceGroup:           "b",
			releaseutil.AnnotationDependsOnResourceGroups: `'["a"]'`,
		}),
	})
	root := newChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "child", Version: "0.1.0", Repository: "file://charts/child"},
	}
	root.SetDependencies(child)

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.ErrorSev, "resource-group circular dependency detected")
	requireMessage(t, messages, support.ErrorSev, "testchart/charts/child")
}

func TestSequencing_IsolatedGroupWarns(t *testing.T) {
	t.Parallel()

	// Two groups, no depends-on edges between them: runtime demotes both to
	// the unsequenced batch with a warning. Lint surfaces that demotion as a
	// WARNING (not an error — the chart still deploys).
	messages := runSequencingLint(t, newChart("testchart", map[string]string{
		"templates/a.yaml": manifestYAML("ConfigMap", "cm-a", map[string]string{releaseutil.AnnotationResourceGroup: "a"}),
		"templates/b.yaml": manifestYAML("ConfigMap", "cm-b", map[string]string{releaseutil.AnnotationResourceGroup: "b"}),
	}))
	requireMessage(t, messages, support.WarningSev, "isolated")
}

func TestSequencing_UndeclaredSubchartWarns(t *testing.T) {
	t.Parallel()

	// Vendored subchart present in charts/ but absent from Chart.yaml
	// dependencies: runtime deploys it after declared subcharts with a
	// warning. Lint mirrors that as a WARNING.
	sub := newChart("vendored", map[string]string{
		"templates/cm.yaml": manifestYAML("ConfigMap", "vendored-cm", nil),
	})
	root := newChart("testchart", nil)
	root.SetDependencies(sub) // deliberately NOT in root.Metadata.Dependencies

	messages := runSequencingLint(t, root)
	requireMessage(t, messages, support.WarningSev, "not declared in Chart.yaml")
}

func TestSequencing_RenderedAnnotationRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		templates     map[string]string
		wantSeverity  int
		wantSubstring string
	}{
		{
			name: "partial readiness annotations",
			templates: map[string]string{
				"templates/configmap.yaml": manifestYAML("ConfigMap", "partial-readiness", map[string]string{
					releaseutil.AnnotationResourceGroup: "bootstrap",
					"helm.sh/readiness-success":         `'["{.phase} == \"Ready\""]'`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "duplicate group assignment",
			templates: map[string]string{
				"templates/first.yaml":  manifestYAML("ConfigMap", "shared-resource", map[string]string{releaseutil.AnnotationResourceGroup: "bootstrap"}),
				"templates/second.yaml": manifestYAML("ConfigMap", "shared-resource", map[string]string{releaseutil.AnnotationResourceGroup: "app"}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "assigned to multiple resource groups",
		},
		{
			name: "circular resource-group dependencies",
			templates: map[string]string{
				"templates/group-a.yaml": manifestYAML("ConfigMap", "group-a", map[string]string{
					releaseutil.AnnotationResourceGroup:           "a",
					releaseutil.AnnotationDependsOnResourceGroups: `'["c"]'`,
				}),
				"templates/group-b.yaml": manifestYAML("ConfigMap", "group-b", map[string]string{
					releaseutil.AnnotationResourceGroup:           "b",
					releaseutil.AnnotationDependsOnResourceGroups: `'["a"]'`,
				}),
				"templates/group-c.yaml": manifestYAML("ConfigMap", "group-c", map[string]string{
					releaseutil.AnnotationResourceGroup:           "c",
					releaseutil.AnnotationDependsOnResourceGroups: `'["b"]'`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "resource-group circular dependency detected",
		},
		{
			name: "orphan resource-group reference",
			templates: map[string]string{
				"templates/configmap.yaml": manifestYAML("ConfigMap", "orphaned-group", map[string]string{
					releaseutil.AnnotationResourceGroup:           "app",
					releaseutil.AnnotationDependsOnResourceGroups: `'["database"]'`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: `depends-on non-existent group "database"`,
		},
		{
			name: "malformed depends-on-resource-groups annotation",
			templates: map[string]string{
				"templates/configmap.yaml": manifestYAML("ConfigMap", "bad-json", map[string]string{
					releaseutil.AnnotationResourceGroup:           "app",
					releaseutil.AnnotationDependsOnResourceGroups: `not-a-json-array`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "depends-on",
		},
		{
			// Both readiness annotations present (so the presence-symmetry
			// check passes), but the JSONPath in readiness-success is
			// malformed. Presence symmetry alone let this through; the rule
			// must validate the expressions compile.
			name: "malformed readiness JSONPath with both annotations present",
			templates: map[string]string{
				"templates/job.yaml": manifestYAML("Job", "bad-jsonpath", map[string]string{
					"helm.sh/readiness-success": `'["{.status.succeeded >= 1"]'`,
					"helm.sh/readiness-failure": `'["{.status.failed} >= 1"]'`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "malformed",
		},
		{
			// The comparison value's type IS statically known: an ordering
			// operator with a non-numeric literal can never evaluate at
			// runtime, so lint flags it as a definite authoring error.
			name: "ordering operator with non-numeric readiness value",
			templates: map[string]string{
				"templates/job.yaml": manifestYAML("Job", "string-ordering", map[string]string{
					"helm.sh/readiness-success": `'["{.status.phase} > \"Running\""]'`,
					"helm.sh/readiness-failure": `'["{.status.failed} >= 1"]'`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "requires a numeric comparison value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			messages := runSequencingLint(t, newChart("testchart", tc.templates))
			requireMessage(t, messages, tc.wantSeverity, tc.wantSubstring)
		})
	}
}

func TestSequencing_HookResourcesExcludedFromResourceGroupDAG(t *testing.T) {
	t.Parallel()

	// A hook resource carrying sequencing annotations that WOULD form a cycle
	// with the non-hook resources must be ignored by the sequencing lint rules,
	// exactly as the install path ignores it (SortManifests routes hooks out
	// before resource-group parsing). group-b depends on group-a; the hook
	// claims group-a depends-on group-b — a cycle only if the hook is counted.
	templates := map[string]string{
		"templates/group-a.yaml": manifestYAML("ConfigMap", "group-a", map[string]string{
			releaseutil.AnnotationResourceGroup: "group-a",
		}),
		"templates/group-b.yaml": manifestYAML("ConfigMap", "group-b", map[string]string{
			releaseutil.AnnotationResourceGroup:           "group-b",
			releaseutil.AnnotationDependsOnResourceGroups: `'["group-a"]'`,
		}),
		"templates/hook-job.yaml": manifestYAML("Job", "hook-job", map[string]string{
			"helm.sh/hook":                                "pre-install",
			releaseutil.AnnotationResourceGroup:           "group-a",
			releaseutil.AnnotationDependsOnResourceGroups: `'["group-b"]'`,
		}),
	}

	messages := runSequencingLint(t, newChart("hook-chart", templates))

	for _, m := range messages {
		assert.NotContains(t, m.Err.Error(), "circular",
			"hook resource must be excluded from the resource-group DAG; got: %v", m.Err)
	}
}

func TestSequencing_NoMessagesWithoutSequencingAnnotations(t *testing.T) {
	t.Parallel()

	messages := runSequencingLint(t, newChart("plain-chart", map[string]string{
		"templates/configmap.yaml": manifestYAML("ConfigMap", "plain", nil),
	}))

	assert.Empty(t, messages)
}

func runSequencingLint(t *testing.T, c *chart.Chart) []support.Message {
	t.Helper()

	tmpDir := t.TempDir()
	require.NoError(t, chartutil.SaveDir(c, tmpDir))

	linter := support.Linter{ChartDir: filepath.Join(tmpDir, c.Name())}
	Sequencing(&linter, "test-namespace", nil)

	return linter.Messages
}

func requireMessage(t *testing.T, messages []support.Message, severity int, substring string) {
	t.Helper()

	for _, message := range messages {
		if message.Severity == severity && strings.Contains(message.Err.Error(), substring) {
			return
		}
	}

	t.Fatalf("expected severity %d message containing %q, got %#v", severity, substring, messages)
}

func newChart(name string, templates map[string]string) *chart.Chart {
	files := make([]*common.File, 0, len(templates))
	names := make([]string, 0, len(templates))
	for templateName := range templates {
		names = append(names, templateName)
	}
	sort.Strings(names)

	for _, templateName := range names {
		files = append(files, &common.File{
			Name: templateName,
			Data: []byte(templates[templateName]),
		})
	}

	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       name,
			Version:    "0.1.0",
			APIVersion: chart.APIVersionV2,
		},
		Templates: files,
	}
}

func manifestYAML(kind, name string, annotations map[string]string) string {
	if len(annotations) == 0 {
		return fmt.Sprintf(`apiVersion: v1
kind: %s
metadata:
  name: %s
data:
  key: value
`, kind, name)
	}

	keys := make([]string, 0, len(annotations))
	for key := range annotations {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var annotationBlock strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&annotationBlock, "\n    %s: %s", key, annotations[key])
	}

	return fmt.Sprintf(`apiVersion: v1
kind: %s
metadata:
  name: %s
  annotations:%s
data:
  key: value
`, kind, name, annotationBlock.String())
}
