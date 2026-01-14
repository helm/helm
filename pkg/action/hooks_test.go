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
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
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
	var commaSeparated strings.Builder
	for i, policy := range hookDefinitions {
		if i+1 == len(hookDefinitions) {
			commaSeparated.WriteString(policy.String())
		} else {
			commaSeparated.WriteString(policy.String() + ",")
		}
	}
	return commaSeparated.String()
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
	t.Helper()
	var expectedOutput string
	if shouldOutput {
		expectedOutput = fmt.Sprintf("attempted to output logs for namespace: %s", expectedNamespace)
	}
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	outBuffer := &bytes.Buffer{}
	instAction.cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: outBuffer}

	modTime := time.Now()
	templates := []*common.File{
		{Name: "templates/hello", ModTime: modTime, Data: []byte("hello: world")},
		{Name: "templates/hooks", ModTime: modTime, Data: []byte(manifest)},
	}
	vals := map[string]interface{}{}

	resi, err := instAction.Run(buildChartWithTemplates(templates), vals)
	is.NoError(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Equal(expectedOutput, outBuffer.String())
	is.Equal(rcommon.StatusDeployed, res.Info.Status)
}

func runInstallForHooksWithFailure(t *testing.T, manifest, expectedNamespace string, shouldOutput bool) {
	t.Helper()
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

	modTime := time.Now()
	templates := []*common.File{
		{Name: "templates/hello", ModTime: modTime, Data: []byte("hello: world")},
		{Name: "templates/hooks", ModTime: modTime, Data: []byte(manifest)},
	}
	vals := map[string]interface{}{}

	resi, err := instAction.Run(buildChartWithTemplates(templates), vals)
	is.Error(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Contains(res.Info.Description, "failed pre-install")
	is.Equal(expectedOutput, outBuffer.String())
	is.Equal(rcommon.StatusFailed, res.Info.Status)
}

type HookFailedError struct{}

func (e *HookFailedError) Error() string {
	return "Hook failed!"
}

type HookFailingKubeClient struct {
	kubefake.PrintingKubeClient
	failOn       resource.Info
	deleteRecord []resource.Info
}

type HookFailingKubeWaiter struct {
	*kubefake.PrintingKubeWaiter
	failOn resource.Info
}

func (*HookFailingKubeClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
	configMap := &v1.ConfigMap{}

	err := yaml.NewYAMLOrJSONDecoder(reader, 1000).Decode(configMap)

	if err != nil {
		return kube.ResourceList{}, err
	}

	return kube.ResourceList{{
		Name:      configMap.Name,
		Namespace: configMap.Namespace,
	}}, nil
}

func (h *HookFailingKubeWaiter) WatchUntilReady(resources kube.ResourceList, _ time.Duration) error {
	for _, res := range resources {
		if res.Name == h.failOn.Name && res.Namespace == h.failOn.Namespace {
			return &HookFailedError{}
		}
	}
	return nil
}

func (h *HookFailingKubeClient) Delete(resources kube.ResourceList, deletionPropagation metav1.DeletionPropagation) (*kube.Result, []error) {
	for _, res := range resources {
		h.deleteRecord = append(h.deleteRecord, resource.Info{
			Name:      res.Name,
			Namespace: res.Namespace,
		})
	}

	return h.PrintingKubeClient.Delete(resources, deletionPropagation)
}

func (h *HookFailingKubeClient) GetWaiterWithOptions(strategy kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	waiter, _ := h.PrintingKubeClient.GetWaiterWithOptions(strategy, opts...)
	return &HookFailingKubeWaiter{
		PrintingKubeWaiter: waiter.(*kubefake.PrintingKubeWaiter),
		failOn:             h.failOn,
	}, nil
}

func TestHooksCleanUp(t *testing.T) {
	hookEvent := release.HookPreInstall

	testCases := []struct {
		name                 string
		inputRelease         release.Release
		failOn               resource.Info
		expectedDeleteRecord []resource.Info
		expectError          bool
	}{
		{
			"Deletion hook runs for previously successful hook on failure of a heavier weight hook",
			release.Release{
				Name:      "test-release",
				Namespace: "test",
				Hooks: []*release.Hook{
					{
						Name: "hook-1",
						Kind: "ConfigMap",
						Path: "templates/service_account.yaml",
						Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: build-config-1
  namespace: test
data:
  foo: bar
`,
						Weight: -5,
						Events: []release.HookEvent{
							hookEvent,
						},
						DeletePolicies: []release.HookDeletePolicy{
							release.HookBeforeHookCreation,
							release.HookSucceeded,
							release.HookFailed,
						},
						LastRun: release.HookExecution{
							Phase: release.HookPhaseSucceeded,
						},
					},
					{
						Name: "hook-2",
						Kind: "ConfigMap",
						Path: "templates/job.yaml",
						Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: build-config-2
  namespace: test
data:
  foo: bar
`,
						Weight: 0,
						Events: []release.HookEvent{
							hookEvent,
						},
						DeletePolicies: []release.HookDeletePolicy{
							release.HookBeforeHookCreation,
							release.HookSucceeded,
							release.HookFailed,
						},
						LastRun: release.HookExecution{
							Phase: release.HookPhaseFailed,
						},
					},
				},
			}, resource.Info{
				Name:      "build-config-2",
				Namespace: "test",
			}, []resource.Info{
				{
					// This should be in the record for `before-hook-creation`
					Name:      "build-config-1",
					Namespace: "test",
				},
				{
					// This should be in the record for `before-hook-creation`
					Name:      "build-config-2",
					Namespace: "test",
				},
				{
					// This should be in the record for cleaning up (the failure first)
					Name:      "build-config-2",
					Namespace: "test",
				},
				{
					// This should be in the record for cleaning up (then the previously successful)
					Name:      "build-config-1",
					Namespace: "test",
				},
			}, true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := &HookFailingKubeClient{
				kubefake.PrintingKubeClient{Out: io.Discard}, tc.failOn, []resource.Info{},
			}

			configuration := &Configuration{
				Releases:     storage.Init(driver.NewMemory()),
				KubeClient:   kubeClient,
				Capabilities: common.DefaultCapabilities,
			}

			serverSideApply := true
			err := configuration.execHook(&tc.inputRelease, hookEvent, kube.StatusWatcherStrategy, nil, 600, serverSideApply)

			if !reflect.DeepEqual(kubeClient.deleteRecord, tc.expectedDeleteRecord) {
				t.Fatalf("Got unexpected delete record, expected: %#v, but got: %#v", kubeClient.deleteRecord, tc.expectedDeleteRecord)
			}

			if err != nil && !tc.expectError {
				t.Fatalf("Got an unexpected error.")
			}

			if err == nil && tc.expectError {
				t.Fatalf("Expected and error but did not get it.")
			}
		})
	}
}

func TestConfiguration_hookSetDeletePolicy(t *testing.T) {
	tests := map[string]struct {
		policies []release.HookDeletePolicy
		expected []release.HookDeletePolicy
	}{
		"no polices specified result in the default policy": {
			policies: nil,
			expected: []release.HookDeletePolicy{
				release.HookBeforeHookCreation,
			},
		},
		"unknown policy is untouched": {
			policies: []release.HookDeletePolicy{
				release.HookDeletePolicy("never"),
			},
			expected: []release.HookDeletePolicy{
				release.HookDeletePolicy("never"),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := &Configuration{}
			h := &release.Hook{
				DeletePolicies: tt.policies,
			}
			cfg.hookSetDeletePolicy(h)
			assert.Equal(t, tt.expected, h.DeletePolicies)
		})
	}
}

func TestExecHook_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)

	failer := &kubefake.FailingKubeClient{
		PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard},
	}

	configuration := &Configuration{
		Releases:     storage.Init(driver.NewMemory()),
		KubeClient:   failer,
		Capabilities: common.DefaultCapabilities,
	}

	rel := &release.Release{
		Name:      "test-release",
		Namespace: "test",
		Hooks: []*release.Hook{
			{
				Name: "test-hook",
				Kind: "ConfigMap",
				Path: "templates/hook.yaml",
				Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-hook
  namespace: test
data:
  foo: bar
`,
				Weight: 0,
				Events: []release.HookEvent{
					release.HookPreInstall,
				},
			},
		},
	}

	// Use WithWaitContext as a marker WaitOption that we can track
	ctx := context.Background()
	waitOptions := []kube.WaitOption{kube.WithWaitContext(ctx)}

	err := configuration.execHook(rel, release.HookPreInstall, kube.StatusWatcherStrategy, waitOptions, 600, false)
	is.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}
