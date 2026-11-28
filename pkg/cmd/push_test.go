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
	"helm.sh/helm/v4/pkg/chart/common"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestPushFileCompletion(t *testing.T) {
	checkFileCompletion(t, "push", true)
	checkFileCompletion(t, "push package.tgz", false)
	checkFileCompletion(t, "push package.tgz oci://localhost:5000", false)
}

// runPushCommand pushes a chart archive to a throwaway OCI registry and
// returns what the command wrote to stdout and stderr.
func runPushCommand(t *testing.T, extraArgs ...string) (string, string) {
	t.Helper()

	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	t.Cleanup(func() { srv.Stop() })

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	require.NoError(t, err)
	ociSrv.Run(t)

	args := append([]string{
		"push", "testdata/testcharts/compressedchart-0.1.0.tgz",
		fmt.Sprintf("oci://%s/u/ocitestuser", ociSrv.RegistryURL),
		"--plain-http",
		"--username", ociSrv.TestUsername,
		"--password", ociSrv.TestPassword,
		"--registry-config", filepath.Join(srv.Root(), "config.json"),
	}, extraArgs...)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	actionConfig := &action.Configuration{
		Releases:     storageFixture(),
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: common.DefaultCapabilities,
	}

	root, err := newRootCmdWithConfig(actionConfig, stdout, args, SetupLogging)
	require.NoError(t, err)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	require.NoError(t, root.Execute(), "stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	return stdout.String(), stderr.String()
}

func TestPushDefaultOutputUnchanged(t *testing.T) {
	defer resetEnv()()

	stdout, _ := runPushCommand(t)

	require.Contains(t, stdout, "Pushed:")
	require.Contains(t, stdout, "Digest: sha256:")
}

func TestPushJSONOutputCarriesDigests(t *testing.T) {
	defer resetEnv()()

	stdout, _ := runPushCommand(t, "--output", "json")

	// The plain-text progress must not reach stdout, or the document cannot
	// be parsed by a caller.
	require.NotContains(t, stdout, "Pushed:")
	require.NotContains(t, stdout, "Digest:")

	var result struct {
		Ref      string `json:"ref"`
		Manifest struct {
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"manifest"`
		Chart struct {
			Digest string `json:"digest"`
		} `json:"chart"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &result), "stdout:\n%s", stdout)

	require.Contains(t, result.Ref, "compressedchart")
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, result.Manifest.Digest)
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, result.Chart.Digest)
	require.NotZero(t, result.Manifest.Size)
}

func TestPushYAMLOutputCarriesDigests(t *testing.T) {
	defer resetEnv()()

	stdout, _ := runPushCommand(t, "--output", "yaml")

	require.NotContains(t, stdout, "Pushed:")

	var result struct {
		Ref      string `yaml:"ref"`
		Manifest struct {
			Digest string `yaml:"digest"`
		} `yaml:"manifest"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(stdout), &result), "stdout:\n%s", stdout)

	require.Contains(t, result.Ref, "compressedchart")
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, result.Manifest.Digest)
}
