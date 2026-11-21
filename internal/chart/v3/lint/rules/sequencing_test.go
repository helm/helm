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

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	"helm.sh/helm/v4/pkg/chart/common"
)

func TestSequencing_SubchartCycle(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", DependsOn: []string{"subchart-b"}},
		{Name: "subchart-b", Version: "0.1.0", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newSequencingChart("subchart-a", nil), newSequencingChart("subchart-b", nil))

	requireLintMessage(t, runSequencingLint(t, root), support.ErrorSev, "subchart circular dependency detected")
}

func TestSequencing_AliasedSubchartDependsOnOriginalName(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Alias: "aliased-a"},
		{Name: "subchart-b", Version: "0.1.0", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newSequencingChart("subchart-a", nil), newSequencingChart("subchart-b", nil))

	assert.Empty(t, runSequencingLint(t, root))
}

func TestSequencing_AmbiguousDependsOnReported(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{
		{Name: "subchart-a", Version: "0.1.0", Alias: "first"},
		{Name: "subchart-a", Version: "0.1.0", Alias: "second"},
		{Name: "subchart-b", Version: "0.1.0", DependsOn: []string{"subchart-a"}},
	}
	root.SetDependencies(newSequencingChart("subchart-a", nil), newSequencingChart("subchart-b", nil))

	requireLintMessage(t, runSequencingLint(t, root), support.ErrorSev, `ambiguous subchart reference "subchart-a"`)
}

func TestSequencing_ParentAnnotationUnknownRef(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.Metadata.Annotations = map[string]string{
		chartutil.AnnotationDependsOnSubcharts: `["does-not-exist"]`,
	}

	requireLintMessage(t, runSequencingLint(t, root), support.ErrorSev, "unknown or disabled subchart")
}

func TestSequencing_NestedSubchartCycle(t *testing.T) {
	t.Parallel()

	child := newSequencingChart("child", map[string]string{"templates/cm.yaml": manifestYAML("child-cm")})
	child.Metadata.Dependencies = []*chart.Dependency{
		{Name: "grandchild-a", Version: "0.1.0", DependsOn: []string{"grandchild-b"}},
		{Name: "grandchild-b", Version: "0.1.0", DependsOn: []string{"grandchild-a"}},
	}
	child.SetDependencies(newSequencingChart("grandchild-a", nil), newSequencingChart("grandchild-b", nil))
	root := newSequencingChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Version: "0.1.0"}}
	root.SetDependencies(child)

	messages := runSequencingLint(t, root)
	requireLintMessage(t, messages, support.ErrorSev, "subchart circular dependency detected")
	requireLintMessage(t, messages, support.ErrorSev, "testchart/charts/child")
}

func TestSequencing_NestedUnknownDependsOnRef(t *testing.T) {
	t.Parallel()

	child := newSequencingChart("child", map[string]string{"templates/cm.yaml": manifestYAML("child-cm")})
	child.Metadata.Dependencies = []*chart.Dependency{
		{Name: "grandchild", Version: "0.1.0", DependsOn: []string{"missing"}},
	}
	child.SetDependencies(newSequencingChart("grandchild", nil))
	root := newSequencingChart("testchart", nil)
	root.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Version: "0.1.0"}}
	root.SetDependencies(child)

	requireLintMessage(t, runSequencingLint(t, root), support.ErrorSev, `depends-on unknown or disabled subchart "missing"`)
}

func TestSequencing_UndeclaredRenderedSubchart(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.SetDependencies(newSequencingChart("vendored", map[string]string{
		"templates/cm.yaml": manifestYAML("vendored-cm"),
	}))

	requireLintMessage(t, runSequencingLint(t, root), support.WarningSev, "not declared in Chart.yaml")
}

func TestSequencing_HookOnlyUndeclaredSubchartIgnored(t *testing.T) {
	t.Parallel()

	root := newSequencingChart("testchart", nil)
	root.SetDependencies(newSequencingChart("vendored", map[string]string{
		"templates/hook.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: hook\n  annotations:\n    helm.sh/hook: pre-install\n",
	}))

	assert.Empty(t, runSequencingLint(t, root))
}

func runSequencingLint(t *testing.T, chrt *chart.Chart) []support.Message {
	t.Helper()

	tmpDir := t.TempDir()
	require.NoError(t, chartutil.SaveDir(chrt, tmpDir))
	linter := support.Linter{ChartDir: filepath.Join(tmpDir, chrt.Name())}
	Sequencing(&linter, "test-namespace", nil)
	return linter.Messages
}

func requireLintMessage(t *testing.T, messages []support.Message, severity int, substring string) {
	t.Helper()

	for _, message := range messages {
		if message.Severity == severity && strings.Contains(message.Err.Error(), substring) {
			return
		}
	}
	t.Fatalf("expected severity %d message containing %q, got %#v", severity, substring, messages)
}

func newSequencingChart(name string, templates map[string]string) *chart.Chart {
	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	sort.Strings(names)

	files := make([]*common.File, 0, len(templates))
	for _, name := range names {
		files = append(files, &common.File{Name: name, Data: []byte(templates[name])})
	}
	return &chart.Chart{
		Metadata:  &chart.Metadata{Name: name, Version: "0.1.0", APIVersion: chart.APIVersionV3},
		Templates: files,
	}
}

func manifestYAML(name string) string {
	return fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n", name)
}
