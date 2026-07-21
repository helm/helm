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
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestTemplateOCIRegistryMessagesNotOnStdout(t *testing.T) {
	defer resetEnv()()

	stdout, stderr := runOCIChartCommand(t, func(ref, registryConfig, contentCache string) []string {
		return []string{
			"template", "release-name", ref,
			"--version", "0.1.0",
			"--plain-http",
			"--registry-config", registryConfig,
			"--content-cache", contentCache,
		}
	})

	require.NotEmpty(t, stdout)
	require.NotContains(t, stdout, "Pulled:")
	require.NotContains(t, stdout, "Digest:")
	require.Contains(t, stderr, "Pulled:")
	require.Contains(t, stderr, "Digest:")
}

func TestShowOCIRegistryMessagesNotOnStdout(t *testing.T) {
	defer resetEnv()()

	stdout, stderr := runOCIChartCommand(t, func(ref, registryConfig, contentCache string) []string {
		return []string{
			"show", "chart", ref,
			"--version", "0.1.0",
			"--plain-http",
			"--registry-config", registryConfig,
			"--content-cache", contentCache,
		}
	})

	require.NotEmpty(t, stdout)
	require.Contains(t, stdout, "name: oci-dependent-chart")
	require.NotContains(t, stdout, "Pulled:")
	require.NotContains(t, stdout, "Digest:")
	require.Contains(t, stderr, "Pulled:")
	require.Contains(t, stderr, "Digest:")
}

func TestAddRegistryClientUsesProvidedWriter(t *testing.T) {
	writer := &bytes.Buffer{}
	client := action.NewShow(action.ShowChart, &action.Configuration{})

	require.NoError(t, addRegistryClient(writer, client))
}

func runOCIChartCommand(t *testing.T, argsFn func(ref, registryConfig, contentCache string) []string) (string, string) {
	t.Helper()

	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testcharts/*.tgz*"),
	)
	t.Cleanup(func() { srv.Stop() })

	ociSrv, err := repotest.NewOCIServer(t, srv.Root())
	require.NoError(t, err)
	ociSrv.Run(t)

	ref := fmt.Sprintf("oci://%s/u/ocitestuser/oci-dependent-chart", ociSrv.RegistryURL)
	registryConfig := filepath.Join(srv.Root(), "config.json")
	contentCache := t.TempDir()
	args := argsFn(ref, registryConfig, contentCache)

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
