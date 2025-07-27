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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	emptyChart                          = "testdata/testcharts/empty"
	chartPath                           = "testdata/testcharts/subchart"
	chartWithNotes                      = "testdata/testcharts/chart-with-notes"
	chartWithNotesAnd2LevelsOfSubCharts = "testdata/testcharts/chart-with-notes-and-2-levels-of-subcharts"
)

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
			cmd:    fmt.Sprintf("template '%s' --values '%s'", chartPath, filepath.Join(chartPath, "/charts/subchartA/values.yaml")),
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
			// Running `helm template` on a chart that doesn't have notes or subcharts should print the template
			// command's output without notes.
			name:   "helm template on chart without notes or subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default", emptyChart),
			golden: "output/template-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm template --notes` on a chart that doesn't have notes or subcharts should print template
			// command's output without notes.
			name:   "helm template --notes on chart without notes or subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --notes", emptyChart),
			golden: "output/template-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm template --render-subchart-notes` on a chart that doesn't have notes or subcharts should
			// print the template command's output without notes.
			name:   "helm template --render-subchart-notes on chart without notes or subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --render-subchart-notes", emptyChart),
			golden: "output/template-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm template --notes --render-subchart-notes` on a chart that doesn't have notes or subcharts
			// should print the template command's output without notes.
			name:   "helm template --notes --render-subchart-notes on chart without notes or subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --notes --render-subchart-notes", emptyChart),
			golden: "output/template-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm template` on a chart that has notes but no subcharts should print the template command's
			// output without notes, since --notes flag is not enabled.
			name:   "helm template on chart with notes without subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default", chartWithNotes),
			golden: "output/template-with-notes.txt",
		},
		{
			// Running `helm template --notes` on a chart that has notes but no subcharts should print the template
			// command's output with (current chart's) notes.
			name:   "helm template --notes on chart with notes without subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --notes", chartWithNotes),
			golden: "output/template-with-notes-with-flag-notes-enabled.txt",
		},
		{
			// Running `helm template --render-subchart-notes` on a chart that has notes but no subcharts should print
			// the template command's output without notes, since --notes flag is not enabled.
			name:   "helm template --render-subchart-notes on chart with notes without subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --render-subchart-notes", chartWithNotes),
			golden: "output/template-with-notes.txt",
		},
		{
			// Running `helm template --notes --render-subchart-notes` on a chart that has notes but no subcharts should
			// print the template command's output (current chart's) notes, i.e., no subchart's notes as no subcharts.
			name: "helm template --notes --render-subchart-notes on chart with notes without subcharts",
			cmd: fmt.Sprintf("template luffy '%s' --namespace default --notes --render-subchart-notes",
				chartWithNotes),
			golden: "output/template-with-notes-with-flag-notes-enabled.txt",
		},
		{
			// Running `helm template` on a chart that has notes and 2 levels of subcharts should print template
			// command's output without notes, since --notes flag is not enabled.
			name:   "helm template on chart with notes and subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default", chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/template-with-notes-and-subcharts.txt",
		},
		{
			// Running `helm template --notes` on a chart that has notes and 2 levels of subcharts should print template
			// command's output with just root chart's notes, i.e., no subchart's notes as no --render-subchart-notes.
			name:   "helm template --notes on chart with notes and subcharts",
			cmd:    fmt.Sprintf("template luffy '%s' --namespace default --notes", chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/template-with-notes-and-subcharts-with-flag-notes-enabled.txt",
		},
		{
			// Running `helm template --render-subchart-notes` on a chart that has notes and 2 levels of subcharts
			// should print the template command's output without notes, since --notes flag is not enabled.
			name: "helm template --render-subchart-notes on chart with notes and subcharts",
			cmd: fmt.Sprintf("template luffy '%s' --namespace default --render-subchart-notes",
				chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/template-with-notes-and-subcharts.txt",
		},
		{
			// Running `helm template --notes --render-subchart-notes` on a chart that has notes and 2 levels of
			// subcharts should print the template command's output with both the root chart and the subchart's notes.
			name: "helm template --notes --render-subchart-notes on chart with notes and subcharts",
			cmd: fmt.Sprintf("template luffy '%s' --namespace default --notes --render-subchart-notes",
				chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/" +
				"template-with-notes-and-subcharts-with-both-flags-notes-and-render-subchart-notes-enabled.txt",
		},
	}
	runTestCmd(t, tests)
}

// TestTemplateHelpOutput tests the `helm template --help` command's output text. This is required because the
// --render-subchart-notes flag's description is different for the template command from that of install/upgrade
// commands.
func TestTemplateHelpOutput(t *testing.T) {
	const (
		outputFilePath   = "testdata/output/template-help.txt"
		testNamespace    = "test-namespace"
		repositoryCache  = "test-repository-cache-dir"
		repositoryConfig = "test-repository-config.yaml"
		registryConfig   = "test-registry-config.json"
		contentCache     = "test-content-cache"
		gnupgHome        = "test-gpg"
		commandText      = "template --help"
	)

	// Reset the envs and the configs at the end of this test so that the updates wouldn’t affect other tests.
	defer resetEnv()()

	// Read the expected output file.
	expectedOutput, err := os.ReadFile(outputFilePath)
	assert.NoError(t, err, "unexpected error while reading expected output's file %q", outputFilePath)

	// Set the configs that might otherwise change based on the local environment if not explicitly set. Note: These
	// configs are not related to the current test.
	settings.RepositoryCache = repositoryCache
	settings.RepositoryConfig = repositoryConfig
	settings.RegistryConfig = registryConfig
	settings.ContentCache = contentCache
	settings.SetNamespace(testNamespace)
	t.Setenv("GNUPGHOME", gnupgHome)

	// Run the `helm template --help` command and compare the help text.
	_, actualOutput, err := executeActionCommandC(storageFixture(), commandText)
	assert.NoError(t, err, "unexpected error running command %q", commandText)
	assert.Equal(t, string(expectedOutput), actualOutput, "mismatch of output")
}

func TestTemplateVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for template version flag with release name",
		cmd:    fmt.Sprintf("%s __complete template releasename testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for template version flag with generate-name",
		cmd:    fmt.Sprintf("%s __complete template --generate-name testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for template version flag too few args",
		cmd:    fmt.Sprintf("%s __complete template testing/alpine --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for template version flag too many args",
		cmd:    fmt.Sprintf("%s __complete template releasename testing/alpine badarg --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for template version flag invalid chart",
		cmd:    fmt.Sprintf("%s __complete template releasename invalid/invalid --version ''", repoSetup),
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
