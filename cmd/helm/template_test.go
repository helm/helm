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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var chartPath = "testdata/testcharts/subchart"

func TestTemplateCmd(t *testing.T) {
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
			cmd:    fmt.Sprintf("template --api-versions helm.k8s.io/test '%s'", chartPath),
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
	}
	runTestCmd(t, tests)
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

func TestTemplateOutputDir(t *testing.T) {
	is := assert.New(t)
	dir := t.TempDir()
	releaseName := "madra"
	_, out, err := executeActionCommand(fmt.Sprintf("template %s '%s' --output-dir=%s", releaseName, chartPath, dir))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	var existent = [][]string{
		{dir, "subchart", "templates", "service.yaml"},
		{dir, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, "subchart", "templates", "tests", "test-nothing.yaml"},
		{dir, "subchart", "templates", "subdir", "role.yaml"},
		{dir, "subchart", "templates", "subdir", "rolebinding.yaml"},
		{dir, "subchart", "templates", "subdir", "serviceaccount.yaml"},
		{dir, "subchart", "charts", "subcharta", "templates", "service.yaml"},
		{dir, "subchart", "charts", "subchartb", "templates", "service.yaml"},
	}
	for _, s := range existent {
		_, err = os.Stat(filepath.Join(s...))
		is.NoError(err)
	}
	nonexistent := [][]string{
		{dir, "hello", "templates", "empty"},
		{dir, releaseName, "subchart", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-nothing.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "role.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "rolebinding.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "serviceaccount.yaml"},
		{dir, releaseName, "subchart", "charts", "subcharta", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "charts", "subchartb", "templates", "service.yaml"},
	}
	for _, f := range nonexistent {
		_, err = os.Stat(filepath.Join(f...))
		is.True(os.IsNotExist(err))
	}
}

func TestTemplateWithCRDsOutputDir(t *testing.T) {
	is := assert.New(t)
	dir := t.TempDir()
	releaseName := "madra"
	_, out, err := executeActionCommand(fmt.Sprintf("template %s '%s' --output-dir=%s --include-crds", releaseName, chartPath, dir))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	var existent = [][]string{
		{dir, "subchart", "crds", "crdA.yaml"},
	}
	for _, s := range existent {
		_, err = os.Stat(filepath.Join(s...))
		is.NoError(err)
	}
	nonexistent := [][]string{
		{dir, "hello", "templates", "empty"},
		{dir, releaseName, "subchart", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-nothing.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "role.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "rolebinding.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "serviceaccount.yaml"},
		{dir, releaseName, "subchart", "charts", "subcharta", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "charts", "subchartb", "templates", "service.yaml"},
	}
	for _, f := range nonexistent {
		_, err = os.Stat(filepath.Join(f...))
		is.True(os.IsNotExist(err))
	}
}

func TestTemplateOutputDirWithReleaseName(t *testing.T) {
	is := assert.New(t)
	dir := t.TempDir()
	releaseName := "madra"
	_, out, err := executeActionCommand(fmt.Sprintf("template %s '%s' --output-dir=%s --release-name", releaseName, chartPath, dir))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	var existent = [][]string{
		{dir, releaseName, "subchart", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, releaseName, "subchart", "templates", "tests", "test-nothing.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "role.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "rolebinding.yaml"},
		{dir, releaseName, "subchart", "templates", "subdir", "serviceaccount.yaml"},
		{dir, releaseName, "subchart", "charts", "subcharta", "templates", "service.yaml"},
		{dir, releaseName, "subchart", "charts", "subchartb", "templates", "service.yaml"},
	}
	for _, s := range existent {
		_, err = os.Stat(filepath.Join(s...))
		is.NoError(err)
	}
	nonexistent := [][]string{
		{dir, releaseName, "hello", "templates", "empty"},
		{dir, "subchart", "templates", "service.yaml"},
		{dir, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, "subchart", "templates", "tests", "test-nothing.yaml"},
		{dir, "subchart", "templates", "subdir", "role.yaml"},
		{dir, "subchart", "templates", "subdir", "rolebinding.yaml"},
		{dir, "subchart", "templates", "subdir", "serviceaccount.yaml"},
		{dir, "subchart", "charts", "subcharta", "templates", "service.yaml"},
		{dir, "subchart", "charts", "subchartb", "templates", "service.yaml"},
	}
	for _, f := range nonexistent {
		_, err = os.Stat(filepath.Join(f...))
		is.True(os.IsNotExist(err))
	}
}

func TestTemplateOutputDirSkiptest(t *testing.T) {
	is := assert.New(t)
	dir := t.TempDir()
	releaseName := "madra"
	_, out, err := executeActionCommand(fmt.Sprintf("template %s '%s' --output-dir=%s --skip-tests", releaseName, chartPath, dir))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}
	nonexistent := [][]string{
		{dir, "subchart", "templates", "tests", "test-config.yaml"},
		{dir, "subchart", "templates", "tests", "test-nothing.yaml"},
	}
	for _, f := range nonexistent {
		_, err = os.Stat(filepath.Join(f...))
		is.True(os.IsNotExist(err))
	}
}

func TestTemplateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "template", false)
	checkFileCompletion(t, "template --generate-name", true)
	checkFileCompletion(t, "template myname", true)
	checkFileCompletion(t, "template myname mychart", false)
}
