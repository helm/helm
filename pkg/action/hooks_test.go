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
	"io"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type HookFailedError struct{}

func (e *HookFailedError) Error() string {
	return "Hook failed!"
}

type HookFailingKubeClient struct {
	kubefake.PrintingKubeClient
	failOn       resource.Info
	deleteRecord []resource.Info
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

func (h *HookFailingKubeClient) WatchUntilReady(resources kube.ResourceList, duration time.Duration) error {
	for _, res := range resources {
		if res.Name == h.failOn.Name && res.Namespace == h.failOn.Namespace {
			return &HookFailedError{}
		}
	}

	return h.PrintingKubeClient.WatchUntilReady(resources, duration)
}

func (h *HookFailingKubeClient) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	for _, res := range resources {
		h.deleteRecord = append(h.deleteRecord, resource.Info{
			Name:      res.Name,
			Namespace: res.Namespace,
		})
	}

	return h.PrintingKubeClient.Delete(resources)
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
				kubefake.PrintingKubeClient{Out: ioutil.Discard}, tc.failOn, []resource.Info{},
			}

			configuration := &Configuration{
				Releases:     storage.Init(driver.NewMemory()),
				KubeClient:   kubeClient,
				Capabilities: chartutil.DefaultCapabilities,
				Log: func(format string, v ...interface{}) {
					t.Helper()
					if *verbose {
						t.Logf(format, v...)
					}
				},
			}

			err := configuration.execHook(&tc.inputRelease, hookEvent, 600)

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
