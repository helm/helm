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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			messages := runSequencingLint(t, newChart("testchart", tc.templates))
			requireMessage(t, messages, tc.wantSeverity, tc.wantSubstring)
		})
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
