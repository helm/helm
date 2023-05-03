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

package action

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chart"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
)

func podManifestWithOutputLogs(hookDefinitions []release.HookOutputLogPolicy) string {
	hookDefinitionString := convertHooksToCommaSeparated(hookDefinitions)
	return fmt.Sprintf(`kind: Pod
metadata:
 name: finding-sharky,
 annotations:
   "helm.sh/hook": pre-install
   "helm.sh/hook-output-log-policy": %s
spec:
 containers:
 - name: sharky-test
   image: fake-image
   cmd: fake-command`, hookDefinitionString)
}

func podManifestWithOutputLogWithNamespace(hookDefinitions []release.HookOutputLogPolicy) string {
	hookDefinitionString := convertHooksToCommaSeparated(hookDefinitions)
	return fmt.Sprintf(`kind: Pod
metadata:
 name: finding-george
 namespace: sneaky-namespace
 annotations:
   "helm.sh/hook": pre-install
   "helm.sh/hook-output-log-policy": %s
spec:
 containers:
 - name: george-test
   image: fake-image
   cmd: fake-command`, hookDefinitionString)
}

func jobManifestWithOutputLog(hookDefinitions []release.HookOutputLogPolicy) string {
	hookDefinitionString := convertHooksToCommaSeparated(hookDefinitions)
	return fmt.Sprintf(`kind: Job
apiVersion: batch/v1
metadata:
 name: losing-religion
 annotations:
   "helm.sh/hook": pre-install
   "helm.sh/hook-output-log-policy": %s
spec:
  completions: 1
  parallelism: 1
  activeDeadlineSeconds: 30
  template:
    spec:
    containers:
    - name: religion-container
      image: religion-image
      cmd: religion-command`, hookDefinitionString)
}

func jobManifestWithOutputLogWithNamespace(hookDefinitions []release.HookOutputLogPolicy) string {
	hookDefinitionString := convertHooksToCommaSeparated(hookDefinitions)
	return fmt.Sprintf(`kind: Job
apiVersion: batch/v1
metadata:
 name: losing-religion
 namespace: rem-namespace
 annotations:
   "helm.sh/hook": pre-install
   "helm.sh/hook-output-log-policy": %s
spec:
  completions: 1
  parallelism: 1
  activeDeadlineSeconds: 30
  template:
    spec:
    containers:
    - name: religion-container
      image: religion-image
      cmd: religion-command`, hookDefinitionString)
}

func convertHooksToCommaSeparated(hookDefinitions []release.HookOutputLogPolicy) string {
	var commaSeparated string
	for i, policy := range hookDefinitions {
		if i+1 == len(hookDefinitions) {
			commaSeparated += policy.String()
		} else {
			commaSeparated += policy.String() + ","
		}
	}
	return commaSeparated
}

func TestInstallRelease_HookOutputLogsOnFailure(t *testing.T) {
	// Should output on failure with expected namespace if hook-failed is set
	runInstallForHooksWithFailure(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithFailure(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "sneaky-namespace", true)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "rem-namespace", true)

	// Should not output on failure with expected namespace if hook-succeed is set
	runInstallForHooksWithFailure(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "", false)
	runInstallForHooksWithFailure(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "", false)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "", false)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "", false)
}

func TestInstallRelease_HookOutputLogsOnSuccess(t *testing.T) {
	// Should output on success with expected namespace if hook-succeeded is set
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "spaced", true)
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "sneaky-namespace", true)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "spaced", true)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded}), "rem-namespace", true)

	// Should not output on success if hook-failed is set
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "", false)
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "", false)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "", false)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnFailed}), "", false)
}

func TestInstallRelease_HooksOutputLogsOnSuccessAndFailure(t *testing.T) {
	// Should output on success with expected namespace if hook-succeeded and hook-failed is set
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithSuccess(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "sneaky-namespace", true)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithSuccess(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "rem-namespace", true)

	// Should output on failure if hook-succeeded and hook-failed is set
	runInstallForHooksWithFailure(t, podManifestWithOutputLogs([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithFailure(t, podManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "sneaky-namespace", true)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLog([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "spaced", true)
	runInstallForHooksWithFailure(t, jobManifestWithOutputLogWithNamespace([]release.HookOutputLogPolicy{release.HookOutputOnSucceeded, release.HookOutputOnFailed}), "rem-namespace", true)
}

func runInstallForHooksWithSuccess(t *testing.T, manifest, expectedNamespace string, shouldOutput bool) {
	var expectedOutput string
	if shouldOutput {
		expectedOutput = fmt.Sprintf("attempted to output logs for namespace: %s", expectedNamespace)
	}
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	outBuffer := &bytes.Buffer{}
	instAction.cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: outBuffer}

	templates := []*chart.File{
		{Name: "templates/hello", Data: []byte("hello: world")},
		{Name: "templates/hooks", Data: []byte(manifest)},
	}
	vals := map[string]interface{}{}

	res, err := instAction.Run(buildChartWithTemplates(templates), vals)
	is.NoError(err)
	is.Equal(expectedOutput, outBuffer.String())
	is.Equal(release.StatusDeployed, res.Info.Status)
}

func runInstallForHooksWithFailure(t *testing.T, manifest, expectedNamespace string, shouldOutput bool) {
	var expectedOutput string
	if shouldOutput {
		expectedOutput = fmt.Sprintf("attempted to output logs for namespace: %s", expectedNamespace)
	}
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	failingClient := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failingClient.WatchUntilReadyError = fmt.Errorf("failed watch")
	instAction.cfg.KubeClient = failingClient
	outBuffer := &bytes.Buffer{}
	failingClient.PrintingKubeClient = kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: outBuffer}

	templates := []*chart.File{
		{Name: "templates/hello", Data: []byte("hello: world")},
		{Name: "templates/hooks", Data: []byte(manifest)},
	}
	vals := map[string]interface{}{}

	res, err := instAction.Run(buildChartWithTemplates(templates), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "failed pre-install")
	is.Equal(expectedOutput, outBuffer.String())
	is.Equal(release.StatusFailed, res.Info.Status)
}
