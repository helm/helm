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
	"testing"
)

func TestInstall(t *testing.T) {
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
		// Install, re-use name
		{
			name:   "install and replace release",
			cmd:    "install aeneas testdata/testcharts/empty --replace",
			golden: "output/install-and-replace.txt",
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
		// Install, using the name-template
		{
			name:   "install with name-template",
			cmd:    "install testdata/testcharts/empty --name-template '{{upper \"foobar\"}}'",
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
			golden:    "output/template-lib-chart.txt",
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
	}

	runTestActionCmd(t, tests)
}

func TestInstallOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "install")
}
