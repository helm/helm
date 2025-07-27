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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v4/pkg/repo/v1/repotest"

	"github.com/stretchr/testify/assert"
)

func TestInstall(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
		repotest.WithMiddleware(repotest.BasicAuthMiddleware(t)),
	)
	defer srv.Stop()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir(srv.Root())).ServeHTTP(w, r)
	}))
	defer srv2.Close()

	if err := srv.LinkIndices(); err != nil {
		t.Fatal(err)
	}

	repoFile := filepath.Join(srv.Root(), "repositories.yaml")

	tests := []cmdTestCase{
		// Install, base case
		{
			name:   "basic install",
			cmd:    "install aeneas testdata/testcharts/empty --namespace default",
			golden: "output/install.txt",
		},

		// Install, values from cli
		{
			name:   "install with values",
			cmd:    "install virgil testdata/testcharts/alpine --set test.Name=bar",
			golden: "output/install-with-values.txt",
		},
		// Install, values from cli via multiple --set
		{
			name:   "install with multiple values",
			cmd:    "install virgil testdata/testcharts/alpine --set test.Color=yellow --set test.Name=banana",
			golden: "output/install-with-multiple-values.txt",
		},
		// Install, values from yaml
		{
			name:   "install with values file",
			cmd:    "install virgil testdata/testcharts/alpine -f testdata/testcharts/alpine/extra_values.yaml",
			golden: "output/install-with-values-file.txt",
		},
		// Install, no hooks
		{
			name:   "install without hooks",
			cmd:    "install aeneas testdata/testcharts/alpine --no-hooks --set test.Name=hello",
			golden: "output/install-no-hooks.txt",
		},
		// Install, values from multiple yaml
		{
			name:   "install with values",
			cmd:    "install virgil testdata/testcharts/alpine -f testdata/testcharts/alpine/extra_values.yaml -f testdata/testcharts/alpine/more_values.yaml",
			golden: "output/install-with-multiple-values-files.txt",
		},
		// Install, no charts
		{
			name:      "install with no chart specified",
			cmd:       "install",
			golden:    "output/install-no-args.txt",
			wantError: true,
		},
		// Install, reuse name
		{
			name:   "install and replace release",
			cmd:    "install aeneas testdata/testcharts/empty --replace",
			golden: "output/install-and-replace.txt",
		},
		// Install, take ownership
		{
			name:   "install and replace release",
			cmd:    "install aeneas-take-ownership testdata/testcharts/empty --take-ownership",
			golden: "output/install-and-take-ownership.txt",
		},
		// Install, with timeout
		{
			name:   "install with a timeout",
			cmd:    "install foobar testdata/testcharts/empty --timeout 120s",
			golden: "output/install-with-timeout.txt",
		},
		// Install, with wait
		{
			name:   "install with a wait",
			cmd:    "install apollo testdata/testcharts/empty --wait",
			golden: "output/install-with-wait.txt",
		},
		// Install, with wait-for-jobs
		{
			name:   "install with wait-for-jobs",
			cmd:    "install apollo testdata/testcharts/empty --wait --wait-for-jobs",
			golden: "output/install-with-wait-for-jobs.txt",
		},
		// Install, using the name-template
		{
			name:   "install with name-template",
			cmd:    "install testdata/testcharts/empty --name-template '{{ \"foobar\"}}'",
			golden: "output/install-name-template.txt",
		},
		// Install, perform chart verification along the way.
		{
			name:      "install with verification, missing provenance",
			cmd:       "install bogus testdata/testcharts/compressedchart-0.1.0.tgz --verify --keyring testdata/helm-test-key.pub",
			wantError: true,
		},
		{
			name:      "install with verification, directory instead of file",
			cmd:       "install bogus testdata/testcharts/signtest --verify --keyring testdata/helm-test-key.pub",
			wantError: true,
		},
		{
			name: "install with verification, valid",
			cmd:  "install signtest testdata/testcharts/signtest-0.1.0.tgz --verify --keyring testdata/helm-test-key.pub",
		},
		// Install, chart with missing dependencies in /charts
		{
			name:      "install chart with missing dependencies",
			cmd:       "install nodeps testdata/testcharts/chart-missing-deps",
			wantError: true,
		},
		// Install chart with update-dependency
		{
			name:   "install chart with missing dependencies",
			cmd:    "install --dependency-update updeps testdata/testcharts/chart-with-subchart-update",
			golden: "output/chart-with-subchart-update.txt",
		},
		// Install, chart with bad dependencies in Chart.yaml in /charts
		{
			name:      "install chart with bad dependencies in Chart.yaml",
			cmd:       "install badreq testdata/testcharts/chart-bad-requirements",
			wantError: true,
		},
		// Install, chart with library chart dependency
		{
			name: "install chart with library chart dependency",
			cmd:  "install withlibchartp testdata/testcharts/chart-with-lib-dep",
		},
		// Install, library chart
		{
			name:      "install library chart",
			cmd:       "install libchart testdata/testcharts/lib-chart",
			wantError: true,
			golden:    "output/install-lib-chart.txt",
		},
		// Install, chart with bad type
		{
			name:      "install chart with bad type",
			cmd:       "install badtype testdata/testcharts/chart-bad-type",
			wantError: true,
			golden:    "output/install-chart-bad-type.txt",
		},
		// Install, values from yaml, schematized
		{
			name:   "install with schema file",
			cmd:    "install schema testdata/testcharts/chart-with-schema",
			golden: "output/schema.txt",
		},
		// Install, values from yaml, schematized with errors
		{
			name:      "install with schema file, with errors",
			cmd:       "install schema testdata/testcharts/chart-with-schema-negative",
			wantError: true,
			golden:    "output/schema-negative.txt",
		},
		// Install, values from yaml, extra values from yaml, schematized with errors
		{
			name:      "install with schema file, extra values from yaml, with errors",
			cmd:       "install schema testdata/testcharts/chart-with-schema -f testdata/testcharts/chart-with-schema/extra-values.yaml",
			wantError: true,
			golden:    "output/schema-negative.txt",
		},
		// Install, values from yaml, extra values from cli, schematized with errors
		{
			name:      "install with schema file, extra values from cli, with errors",
			cmd:       "install schema testdata/testcharts/chart-with-schema --set age=-5",
			wantError: true,
			golden:    "output/schema-negative-cli.txt",
		},
		// Install with subchart, values from yaml, schematized with errors
		{
			name:      "install with schema file and schematized subchart, with errors",
			cmd:       "install schema testdata/testcharts/chart-with-schema-and-subchart",
			wantError: true,
			golden:    "output/subchart-schema-negative.txt",
		},
		// Install with subchart, values from yaml, extra values from cli, schematized with errors
		{
			name:   "install with schema file and schematized subchart, extra values from cli",
			cmd:    "install schema testdata/testcharts/chart-with-schema-and-subchart --set lastname=doe --set subchart-with-schema.age=25",
			golden: "output/subchart-schema-cli.txt",
		},
		// Install with subchart, values from yaml, extra values from cli, schematized with errors
		{
			name:      "install with schema file and schematized subchart, extra values from cli, with errors",
			cmd:       "install schema testdata/testcharts/chart-with-schema-and-subchart --set lastname=doe --set subchart-with-schema.age=-25",
			wantError: true,
			golden:    "output/subchart-schema-cli-negative.txt",
		},
		// Install, values from yaml, schematized with errors but skip schema validation, expect success
		{
			name:   "install with schema file and schematized subchart, extra values from cli, skip schema validation",
			cmd:    "install schema testdata/testcharts/chart-with-schema-and-subchart --set lastname=doe --set subchart-with-schema.age=-25 --skip-schema-validation",
			golden: "output/schema.txt",
		},
		// Install deprecated chart
		{
			name:   "install with warning about deprecated chart",
			cmd:    "install aeneas testdata/testcharts/deprecated --namespace default",
			golden: "output/deprecated-chart.txt",
		},
		// Install chart with only crds
		{
			name: "install chart with only crds",
			cmd:  "install crd-test testdata/testcharts/chart-with-only-crds --namespace default",
		},
		// Verify the user/pass works
		{
			name:   "basic install with credentials",
			cmd:    "install aeneas reqtest --namespace default --repo " + srv.URL() + " --username username --password password",
			golden: "output/install.txt",
		},
		{
			name:   "basic install with credentials",
			cmd:    "install aeneas reqtest --namespace default --repo " + srv2.URL + " --username username --password password --pass-credentials",
			golden: "output/install.txt",
		},
		{
			name:   "basic install with credentials and no repo",
			cmd:    fmt.Sprintf("install aeneas test/reqtest --username username --password password --repository-config %s --repository-cache %s", repoFile, srv.Root()),
			golden: "output/install.txt",
		},
		{
			name:   "dry-run displaying secret",
			cmd:    "install secrets testdata/testcharts/chart-with-secret --dry-run",
			golden: "output/install-dry-run-with-secret.txt",
		},
		{
			name:   "dry-run hiding secret",
			cmd:    "install secrets testdata/testcharts/chart-with-secret --dry-run --hide-secret",
			golden: "output/install-dry-run-with-secret-hidden.txt",
		},
		{
			name:      "hide-secret error without dry-run",
			cmd:       "install secrets testdata/testcharts/chart-with-secret --hide-secret",
			wantError: true,
			golden:    "output/install-hide-secret.txt",
		},
		{
			// Running `helm install` on a chart that doesn't have notes or subcharts should print the install command's
			// output without notes.
			name:   "helm install on chart without notes or subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default", emptyChart),
			golden: "output/install-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm install --dry-run` on a chart that doesn't have notes or subcharts should print the install
			// command's dry-run output without notes.
			name:   "helm install --dry-run on chart without notes or subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default --dry-run", emptyChart),
			golden: "output/install-without-notes-or-subcharts-with-flag-dry-run-enabled.txt",
		},
		{
			// Running `helm install --render-subchart-notes` on a chart that doesn't have notes or subcharts should
			// print the install command's output without any notes.
			name:   "helm install --render-subchart-notes on chart without notes or subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default --render-subchart-notes", emptyChart),
			golden: "output/install-without-notes-or-subcharts.txt",
		},
		{
			// Running `helm install --dry-run --render-subchart-notes` on a chart that doesn't have notes or subcharts
			// should print the install command's dry-run output without any notes.
			name:   "helm install --dry-run --render-subchart-notes on chart without notes or subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default --dry-run --render-subchart-notes", emptyChart),
			golden: "output/install-without-notes-or-subcharts-with-flag-dry-run-enabled.txt",
		},
		{
			// Running `helm install` on a chart that has notes but no subcharts should print the install command's
			// output with (current chart's) notes.
			name:   "helm install on chart with notes without subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default", chartWithNotes),
			golden: "output/install-with-notes.txt",
		},
		{
			// Running `helm install --dry-run` on a chart that has notes but no subcharts should print the install
			// command's dry-run output with (current chart's) notes. Note: The notes in dry-run output include the
			// source filename and separator "---".
			name:   "helm install --dry-run on chart with notes without subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default --dry-run", chartWithNotes),
			golden: "output/install-with-notes-with-flag-dry-run-enabled.txt",
		},
		{
			// Running `helm install --render-subchart-notes` on a chart that has notes but no subcharts should print
			// the install command's output with (current chart's) notes, i.e., no subchart's notes as no subchart.
			name:   "helm install --render-subchart-notes on chart with notes without subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default --render-subchart-notes", chartWithNotes),
			golden: "output/install-with-notes.txt",
		},
		{
			// Running `helm install --dry-run --render-subchart-notes` on a chart that has notes but no subcharts
			// should print the install command's dry-run output (current chart's) notes, i.e., no subchart's notes as
			// no subchart. Note: The notes in dry-run output include the source filename and separator "---".
			name: "helm install --dry-run --render-subchart-notes on chart with notes without subcharts",
			cmd: fmt.Sprintf("install luffy '%s' --namespace default --dry-run --render-subchart-notes",
				chartWithNotes),
			golden: "output/install-with-notes-with-flag-dry-run-enabled.txt",
		},
		{
			// Running `helm install` on a chart that has notes and 2 levels of subcharts should print install command's
			// output with just root chart's notes, i.e., without subchart's notes.
			name:   "helm install on chart with notes and subcharts",
			cmd:    fmt.Sprintf("install luffy '%s' --namespace default", chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/install-with-notes-and-subcharts.txt",
		},
		{
			// Running `helm install --dry-run` on a chart that has notes and 2 levels of subcharts should print the
			// install command's dry-run output with just the root chart's notes, i.e., without the subchart's notes.
			// sNote: The notes in dry-run output include the source filename and separator "---".
			name: "helm install --dry-run on chart with notes and subcharts",
			cmd: fmt.Sprintf("install luffy '%s' --namespace default --dry-run",
				chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/install-with-notes-and-subcharts-with-flag-dry-run-enabled.txt",
		},
		{
			// Running `helm install --render-subchart-notes` on a chart that has notes and 2 levels of subcharts should
			// print the install command's output with both the root chart and the subcharts' notes.
			name: "helm install --render-subchart-notes on chart with notes and subcharts",
			cmd: fmt.Sprintf("install luffy '%s' --namespace default --render-subchart-notes",
				chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/install-with-notes-and-subcharts-with-flag-render-subchart-notes-enabled.txt",
		},
		{
			// Running `helm install --dry-run --render-subchart-notes` on a chart that has notes and 2 levels of
			// subcharts should print the install command's dry-run output with both the root chart and the subcharts'
			// notes. Note: The notes in dry-run output include the source filename and separator "---".
			name: "helm install --dry-run --render-subchart-notes on chart with notes and subcharts",
			cmd: fmt.Sprintf("install luffy '%s' --namespace default --dry-run --render-subchart-notes",
				chartWithNotesAnd2LevelsOfSubCharts),
			golden: "output/" +
				"install-with-notes-and-subcharts-with-both-flags-dry-run-and-render-subchart-notes-enabled.txt",
		},
	}

	runTestCmd(t, tests)
}

// TestInstallHelpOutput tests the `helm install --help` command's output text. This is required because the
// --render-subchart-notes flag's description is different for the template command from that of install/upgrade
// commands.
func TestInstallHelpOutput(t *testing.T) {
	const (
		outputFilePath   = "testdata/output/install-help.txt"
		testNamespace    = "test-namespace"
		repositoryCache  = "test-repository-cache-dir"
		repositoryConfig = "test-repository-config.yaml"
		registryConfig   = "test-registry-config.json"
		contentCache     = "test-content-cache"
		gnupgHome        = "test-gpg"
		commandText      = "install --help"
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

	// Run the `helm install --help` command and compare the help text.
	_, actualOutput, err := executeActionCommandC(storageFixture(), commandText)
	assert.NoError(t, err, "unexpected error running command %q", commandText)
	assert.Equal(t, string(expectedOutput), actualOutput, "mismatch of output")
}

func TestInstallOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "install")
}

func TestInstallVersionCompletion(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	tests := []cmdTestCase{{
		name:   "completion for install version flag with release name",
		cmd:    fmt.Sprintf("%s __complete install releasename testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for install version flag with generate-name",
		cmd:    fmt.Sprintf("%s __complete install --generate-name testing/alpine --version ''", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for install version flag, no filter",
		cmd:    fmt.Sprintf("%s __complete install releasename testing/alpine --version 0.3", repoSetup),
		golden: "output/version-comp.txt",
	}, {
		name:   "completion for install version flag too few args",
		cmd:    fmt.Sprintf("%s __complete install testing/alpine --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for install version flag too many args",
		cmd:    fmt.Sprintf("%s __complete install releasename testing/alpine badarg --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}, {
		name:   "completion for install version flag invalid chart",
		cmd:    fmt.Sprintf("%s __complete install releasename invalid/invalid --version ''", repoSetup),
		golden: "output/version-invalid-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestInstallFileCompletion(t *testing.T) {
	checkFileCompletion(t, "install", false)
	checkFileCompletion(t, "install --generate-name", true)
	checkFileCompletion(t, "install myname", true)
	checkFileCompletion(t, "install myname mychart", false)
}
