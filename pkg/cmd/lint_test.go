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
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/action"
)

func TestLintCmdWithSubchartsFlag(t *testing.T) {
	testChart := "testdata/testcharts/chart-with-bad-subcharts"
	tests := []cmdTestCase{{
		name:      "lint good chart with bad subcharts",
		cmd:       "lint " + testChart,
		golden:    "output/lint-chart-with-bad-subcharts.txt",
		wantError: true,
	}, {
		name:      "lint good chart with bad subcharts using --with-subcharts flag",
		cmd:       "lint --with-subcharts " + testChart,
		golden:    "output/lint-chart-with-bad-subcharts-with-subcharts.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintCmdWithQuietFlag(t *testing.T) {
	testChart1 := "testdata/testcharts/alpine"
	testChart2 := "testdata/testcharts/chart-bad-requirements"
	tests := []cmdTestCase{{
		name:   "lint good chart using --quiet flag",
		cmd:    "lint --quiet " + testChart1,
		golden: "output/lint-quiet.txt",
	}, {
		name:      "lint two charts, one with error using --quiet flag",
		cmd:       fmt.Sprintf("lint --quiet %s %s", testChart1, testChart2),
		golden:    "output/lint-quiet-with-error.txt",
		wantError: true,
	}, {
		name:   "lint chart with warning using --quiet flag",
		cmd:    "lint --quiet testdata/testcharts/chart-with-only-crds",
		golden: "output/lint-quiet-with-warning.txt",
	}, {
		name:      "lint non-existent chart using --quiet flag",
		cmd:       "lint --quiet thischartdoesntexist/",
		golden:    "",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintCmdWithKubeVersionFlag(t *testing.T) {
	testChart := "testdata/testcharts/chart-with-deprecated-api"
	tests := []cmdTestCase{{
		name:      "lint chart with deprecated api version using kube version flag",
		cmd:       "lint --kube-version 1.22.0 " + testChart,
		golden:    "output/lint-chart-with-deprecated-api.txt",
		wantError: false,
	}, {
		name:      "lint chart with deprecated api version using kube version and strict flag",
		cmd:       "lint --kube-version 1.22.0 --strict " + testChart,
		golden:    "output/lint-chart-with-deprecated-api-strict.txt",
		wantError: true,
	}, {
		// the test builds will use the kubeVersionMinorTesting const in capabilities.go
		// which is "20"
		name:      "lint chart with deprecated api version without kube version",
		cmd:       "lint " + testChart,
		golden:    "output/lint-chart-with-deprecated-api-old-k8s.txt",
		wantError: false,
	}, {
		name:      "lint chart with deprecated api version with older kube version",
		cmd:       "lint --kube-version 1.20.0 --strict " + testChart,
		golden:    "output/lint-chart-with-deprecated-api-old-k8s.txt",
		wantError: false,
	}}
	runTestCmd(t, tests)
}

func TestLintCmdRequiresArgs(t *testing.T) {
	tests := []cmdTestCase{{
		name:      "lint without arguments should fail",
		cmd:       "lint",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintCmdWithOutputFormat(t *testing.T) {
	tests := []cmdTestCase{{
		name:      "lint failing chart with --output json",
		cmd:       "lint --output json testdata/testcharts/chart-with-bad-subcharts",
		golden:    "output/lint-output-json.txt",
		wantError: true,
	}, {
		name:   "lint good chart with --output json",
		cmd:    "lint --output json testdata/testcharts/alpine",
		golden: "output/lint-output-json-good.txt",
	}, {
		name:   "lint chart with warning with --output yaml",
		cmd:    "lint --output yaml testdata/testcharts/chart-with-only-crds",
		golden: "output/lint-output-yaml.txt",
	}, {
		name:      "lint two charts with --output json",
		cmd:       "lint --output json testdata/testcharts/alpine testdata/testcharts/chart-with-bad-subcharts",
		golden:    "output/lint-output-json-multiple.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintCmdStructuredStreams(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			defer resetEnv()()
			missing := filepath.Join(t.TempDir(), "missing-chart")
			args := []string{"lint", "-o", format, "--quiet", "testdata/testcharts/alpine", missing}
			var stdout, stderr bytes.Buffer
			root, err := newRootCmdWithConfig(action.NewConfiguration(), &stdout, args, SetupLogging)
			require.NoError(t, err)
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(args)
			require.EqualError(t, root.Execute(), "2 chart(s) linted, 1 chart(s) failed")
			require.Contains(t, stderr.String(), "2 chart(s) linted, 1 chart(s) failed")

			data := stdout.Bytes()
			if format == "yaml" {
				data, err = yaml.YAMLToJSONStrict(data)
				require.NoError(t, err)
			}
			var report struct {
				ChartsLinted int `json:"charts_linted"`
				ChartsFailed int `json:"charts_failed"`
				Results      []struct {
					Chart    string
					Failed   bool
					Messages []map[string]string
				}
			}
			require.NoError(t, json.Unmarshal(data, &report))
			require.Equal(t, 2, report.ChartsLinted)
			require.Equal(t, 1, report.ChartsFailed)
			require.Len(t, report.Results, 1)
			result := report.Results[0]
			require.Equal(t, missing, result.Chart)
			require.True(t, result.Failed)
			require.Len(t, result.Messages, 1)
			finding := result.Messages[0]
			require.Equal(t, "ERROR", finding["severity"])
			require.Contains(t, finding["message"], "Chart.yaml")
			require.NotContains(t, finding, "path")
		})
	}
}

func TestLintCmdTableWriteError(t *testing.T) {
	defer resetEnv()()
	reader, writer := io.Pipe()
	require.NoError(t, reader.Close())
	t.Cleanup(func() { _ = writer.Close() })
	cmd := newLintCmd(writer)
	require.ErrorIs(t, cmd.RunE(cmd, []string{"testdata/testcharts/alpine"}), io.ErrClosedPipe)
}

func TestLintFileCompletion(t *testing.T) {
	checkFileCompletion(t, "lint", true)
	checkFileCompletion(t, "lint mypath", true) // Multiple paths can be given
}
