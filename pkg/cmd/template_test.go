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

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
)

var chartPath = "testdata/testcharts/subchart"

func TestTemplateCmd(t *testing.T) {
	deletevalchart := "testdata/testcharts/issue-9027"

	tests := []cmdTestCase{
		{
			name:   "check name",
			cmd:    fmt.Sprintf("template '%s'", chartPath),
			golden: "output/template.txt",
		},
		{
			name:   "check set name",
			cmd:    fmt.Sprintf("template '%s' --set service.name=apache", chartPath),
			golden: "output/template-set.txt",
		},
		{
			name:   "check values files",
			cmd:    fmt.Sprintf("template '%s' --values '%s'", chartPath, filepath.Join(chartPath, "charts", "subchartA", "values.yaml")),
			golden: "output/template-values-files.txt",
		},
		{
			name:   "check name template",
			cmd:    fmt.Sprintf(`template '%s' --name-template='foobar-{{ b64enc "abc" | lower }}-baz'`, chartPath),
			golden: "output/template-name-template.txt",
		},
		{
			name:      "check no args",
			cmd:       "template",
			wantError: true,
			golden:    "output/template-no-args.txt",
		},
		{
			name:      "check library chart",
			cmd:       fmt.Sprintf("template '%s'", "testdata/testcharts/lib-chart"),
			wantError: true,
			golden:    "output/template-lib-chart.txt",
		},
		{
			name:      "check chart bad type",
			cmd:       fmt.Sprintf("template '%s'", "testdata/testcharts/chart-bad-type"),
			wantError: true,
			golden:    "output/template-chart-bad-type.txt",
		},
		{
			name:   "check chart with dependency which is an app chart acting as a library chart",
			cmd:    fmt.Sprintf("template '%s'", "testdata/testcharts/chart-with-template-lib-dep"),
			golden: "output/template-chart-with-template-lib-dep.txt",
		},
		{
			name:   "check chart with dependency which is an app chart archive acting as a library chart",
			cmd:    fmt.Sprintf("template '%s'", "testdata/testcharts/chart-with-template-lib-archive-dep"),
			golden: "output/template-chart-with-template-lib-archive-dep.txt",
		},
		{
			name:   "check kube version",
			cmd:    fmt.Sprintf("template --kube-version 1.16.0 '%s'", chartPath),
			golden: "output/template-with-kube-version.txt",
		},
		{
			name:   "check kube api versions",
			cmd:    fmt.Sprintf("template --api-versions helm.k8s.io/test,helm.k8s.io/test2 '%s'", chartPath),
			golden: "output/template-with-api-version.txt",
		},
		{
			name:   "check kube api versions",
			cmd:    fmt.Sprintf("template --api-versions helm.k8s.io/test --api-versions helm.k8s.io/test2 '%s'", chartPath),
			golden: "output/template-with-api-version.txt",
		},
		{
			name:   "template with CRDs",
			cmd:    fmt.Sprintf("template '%s' --include-crds", chartPath),
			golden: "output/template-with-crds.txt",
		},
		{
			name:   "template with show-only one",
			cmd:    fmt.Sprintf("template '%s' --show-only templates/service.yaml", chartPath),
			golden: "output/template-show-only-one.txt",
		},
		{
			name:   "template with show-only multiple",
			cmd:    fmt.Sprintf("template '%s' --show-only templates/service.yaml --show-only charts/subcharta/templates/service.yaml", chartPath),
			golden: "output/template-show-only-multiple.txt",
		},
		{
			name:   "template with show-only glob",
			cmd:    fmt.Sprintf("template '%s' --show-only templates/subdir/role*", chartPath),
			golden: "output/template-show-only-glob.txt",
			// Repeat to ensure manifest ordering regressions are caught
			repeat: 10,
		},
		{
			name:   "sorted output of manifests (order of filenames, then order of objects within each YAML file)",
			cmd:    fmt.Sprintf("template '%s'", "testdata/testcharts/object-order"),
			golden: "output/object-order.txt",
			// Helm previously used random file order. Repeat the test so we
			// don't accidentally get the expected result.
			repeat: 10,
		},
		{
			name:      "chart with template with invalid yaml",
			cmd:       fmt.Sprintf("template '%s'", "testdata/testcharts/chart-with-template-with-invalid-yaml"),
			wantError: true,
			golden:    "output/template-with-invalid-yaml.txt",
		},
		{
			name:      "chart with template with invalid yaml (--debug)",
			cmd:       fmt.Sprintf("template '%s' --debug", "testdata/testcharts/chart-with-template-with-invalid-yaml"),
			wantError: true,
			golden:    "output/template-with-invalid-yaml-debug.txt",
		},
		{
			name:      "chart with template with invalid template expression (--debug, --show-only)",
			cmd:       fmt.Sprintf("template '%s' --debug --show-only %s", "testdata/testcharts/chart-with-template-with-invalid-template-expr", "templates/alpine-pod.yaml"),
			wantError: true,
			golden:    "output/template-with-invalid-template-expr-debug-show-only.txt",
		},
		{
			name:   "template skip-tests",
			cmd:    fmt.Sprintf(`template '%s' --skip-tests`, chartPath),
			golden: "output/template-skip-tests.txt",
		},
		{
			// This test case is to ensure the case where specified dependencies
			// in the Chart.yaml and those where the Chart.yaml don't have them
			// specified are the same.
			name:   "ensure nil/null values pass to subcharts delete values",
			cmd:    fmt.Sprintf("template '%s'", deletevalchart),
			golden: "output/issue-9027.txt",
		},
		{
			// Ensure that parent chart values take precedence over imported values
			name:   "template with imported subchart values ensuring import",
			cmd:    fmt.Sprintf("template '%s' --set configmap.enabled=true --set subchartb.enabled=true", chartPath),
			golden: "output/template-subchart-cm.txt",
		},
		{
			// Ensure that user input values take precedence over imported
			// values from sub-charts.
			name:   "template with imported subchart values set with --set",
			cmd:    fmt.Sprintf("template '%s' --set configmap.enabled=true --set subchartb.enabled=true --set configmap.value=baz", chartPath),
			golden: "output/template-subchart-cm-set.txt",
		},
		{
			// Ensure that user input values take precedence over imported
			// values from sub-charts when passed by file
			name:   "template with imported subchart values set with --set",
			cmd:    fmt.Sprintf("template '%s' -f %s/extra_values.yaml", chartPath, chartPath),
			golden: "output/template-subchart-cm-set-file.txt",
		},
		{
			name:   "template with ordered wait strategy shows resource group delimiters",
			cmd:    "template --wait=ordered 'testdata/testcharts/sequenced-chart'",
			golden: "output/template-ordered-delimiters.txt",
		},
		{
			name:   "template with ordered wait strategy demotes isolated groups",
			cmd:    "template --wait=ordered 'testdata/testcharts/sequenced-isolated-chart'",
			golden: "output/template-ordered-isolated.txt",
		},
	}
	runTestCmd(t, tests)
}

func TestTemplateWithoutOrderedWaitHasNoDelimiters(t *testing.T) {
	_, out, err := executeActionCommand("template 'testdata/testcharts/sequenced-chart'")
	require.NoError(t, err)
	require.NotContains(t, out, "## START resource-group:")
	require.NotContains(t, out, "## END resource-group:")
	require.Contains(t, out, "# Source: sequenced-chart/charts/worker/templates/aa-worker-configmap.yaml")
}

// TestTemplateOrderedBackwardsCompat asserts the HIP-0025 backwards-compat
// invariant: for a chart with no sequencing annotations, `helm template` and
// `helm template --wait=ordered` produce the same manifest body (after
// stripping the `## START`/`## END` resource-group delimiter lines that only
// the ordered path emits). This is the unit-test analogue of harness scenario
// S04-05; it regressed twice — once when the ordered path's per-manifest
// emission used tighter inter-document whitespace than the flat path, and once
// when the fix for that left an extra trailing blank line at EOF. The trailing
// whitespace is deliberately NOT trimmed here so this test catches both — the
// shell harness's `$()` substitution masks trailing-newline drift, so a stricter
// comparison belongs at the unit level.
func TestTemplateOrderedBackwardsCompat(t *testing.T) {
	const chartPath = "testdata/testcharts/alpine"
	_, flat, err := executeActionCommand("template t " + chartPath)
	require.NoError(t, err)
	_, ordered, err := executeActionCommand("template t " + chartPath + " --wait=ordered")
	require.NoError(t, err)

	// Strip only the marker lines the ordered path adds; preserve all other
	// whitespace including the trailing newline structure.
	stripMarkers := func(s string) string {
		lines := strings.Split(s, "\n")
		out := lines[:0]
		for _, l := range lines {
			if strings.HasPrefix(l, "## START resource-group:") ||
				strings.HasPrefix(l, "## END resource-group:") {
				continue
			}
			out = append(out, l)
		}
		return strings.Join(out, "\n")
	}

	require.Equal(t, stripMarkers(flat), stripMarkers(ordered),
		"ordered template output must be byte-identical (incl. trailing newline) to flat output for charts without sequencing annotations")
}

// TestTemplateStripsHelmInternalAnnotations asserts that `helm template` output
// never contains the multi-slash internal annotation key
// `helm.sh/depends-on/resource-groups` — its presence in the K8s API would
// fail annotation-key validation and break `helm template | kubectl apply -f -`.
// Valid sibling keys (single-slash) like `helm.sh/resource-group` must survive.
func TestTemplateStripsHelmInternalAnnotations(t *testing.T) {
	const internalKey = "helm.sh/depends-on/resource-groups"
	const siblingKey = "helm.sh/resource-group"

	t.Run("flat path", func(t *testing.T) {
		_, out, err := executeActionCommand("template 'testdata/testcharts/sequenced-chart'")
		require.NoError(t, err)
		require.NotContains(t, out, internalKey, "internal annotation must be stripped from flat template output")
		require.Contains(t, out, siblingKey, "valid-key sibling annotation must be preserved")
	})

	t.Run("ordered path", func(t *testing.T) {
		_, out, err := executeActionCommand("template --wait=ordered 'testdata/testcharts/sequenced-chart'")
		require.NoError(t, err)
		require.NotContains(t, out, internalKey, "internal annotation must be stripped from ordered template output")
		require.Contains(t, out, siblingKey, "valid-key sibling annotation must be preserved")
		require.Contains(t, out, "## START resource-group:", "ordered output must still emit group delimiters")
	})

	t.Run("output-dir path", func(t *testing.T) {
		dir := t.TempDir()
		_, _, err := executeActionCommand(fmt.Sprintf("template 'testdata/testcharts/sequenced-chart' --output-dir '%s'", dir))
		require.NoError(t, err)
		files := readOutputDirManifests(t, dir)
		require.NotContains(t, files, internalKey, "internal annotation must be stripped from --output-dir files")
		require.Contains(t, files, siblingKey, "valid-key sibling annotation must be preserved in --output-dir files")
	})

	t.Run("output-dir hooks path", func(t *testing.T) {
		dir := t.TempDir()
		_, _, err := executeActionCommand(fmt.Sprintf("template 'testdata/testcharts/sequenced-hook-chart' --output-dir '%s'", dir))
		require.NoError(t, err)
		hook, err := os.ReadFile(filepath.Join(dir, "sequenced-hook-chart", "templates", "hook-configmap.yaml"))
		require.NoError(t, err)
		require.NotContains(t, string(hook), internalKey, "internal annotation must be stripped from hook files under --output-dir")
		require.Contains(t, string(hook), siblingKey, "valid-key sibling annotation must be preserved in hook files")
	})
}

// readOutputDirManifests concatenates every file under an --output-dir tree so
// tests can assert on the rendered file contents as a whole.
func readOutputDirManifests(t *testing.T, dir string) string {
	t.Helper()
	var sb strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sb.Write(b)
		sb.WriteString("\n")
		return nil
	})
	require.NoError(t, err)
	return sb.String()
}

func TestTemplateOrderedMatchesPlan(t *testing.T) {
	manifest := `# Source: parent/templates/aa-alpha.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-alpha
  annotations:
    helm.sh/resource-group: alpha
---
# Source: parent/templates/bb-beta.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-beta
  annotations:
    helm.sh/resource-group: beta
    helm.sh/depends-on/resource-groups: '["alpha"]'
---
# Source: parent/templates/cc-gamma.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-gamma
  annotations:
    helm.sh/resource-group: gamma
---
# Source: parent/templates/dd-plain.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-plain
---
# Source: parent/charts/vendored/templates/aa-vendored.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-vendored
  annotations:
    helm.sh/resource-group: vendored
`
	chrt := &chart.Chart{Metadata: &chart.Metadata{Name: "parent"}}
	manifests, err := sequence.ParseStoredManifests(manifest)
	require.NoError(t, err)
	plan, err := sequence.Build(chrt, manifests)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, renderOrderedTemplate(chrt, manifest, &out))
	require.Equal(t, orderedTemplateSequenceFromPlan(plan), orderedTemplateSequenceFromOutput(out.String()))
}

func orderedTemplateSequenceFromPlan(plan *sequence.Plan) []string {
	var ordered []string
	for _, batch := range plan.Batches {
		switch batch.Kind {
		case sequence.BatchKindGroups:
			for _, group := range batch.Groups {
				ordered = append(ordered, "group:"+sequence.DisplayPath(batch.ChartPath)+" "+group.Name)
				for _, manifest := range group.Manifests {
					ordered = append(ordered, "source:"+manifest.Name)
				}
			}
		case sequence.BatchKindUnsequenced:
			for _, manifest := range batch.Manifests() {
				ordered = append(ordered, "source:"+manifest.Name)
			}
		}
	}
	return ordered
}

func orderedTemplateSequenceFromOutput(output string) []string {
	var ordered []string
	for line := range strings.SplitSeq(output, "\n") {
		if marker, ok := strings.CutPrefix(line, "## START resource-group: "); ok {
			ordered = append(ordered, "group:"+marker)
			continue
		}
		if source, ok := strings.CutPrefix(line, "# Source: "); ok {
			ordered = append(ordered, "source:"+source)
		}
	}
	return ordered
}

func TestTemplateVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for template version flag with release name",
		cmd:    repoSetup + " __complete template releasename testing/alpine --version ''",
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for template version flag with generate-name",
		cmd:    repoSetup + " __complete template --generate-name testing/alpine --version ''",
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for template version flag too few args",
		cmd:    repoSetup + " __complete template testing/alpine --version ''",
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for template version flag too many args",
		cmd:    repoSetup + " __complete template releasename testing/alpine badarg --version ''",
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for template version flag invalid chart",
		cmd:    repoSetup + " __complete template releasename invalid/invalid --version ''",
		golden: "output/version-invalid-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestTemplateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "template", false)
	checkFileCompletion(t, "template --generate-name", true)
	checkFileCompletion(t, "template myname", true)
	checkFileCompletion(t, "template myname mychart", false)
}
