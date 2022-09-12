package action

import (
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"io"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"reflect"
	"testing"
	"time"
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

func (_ *HookFailingKubeClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
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
	hookKubeClient := &HookFailingKubeClient{kubefake.PrintingKubeClient{Out: ioutil.Discard}, resource.Info{
		Name:      "build-config-2",
		Namespace: "test",
	}, []resource.Info{}}

	configuration := &Configuration{
		Releases:     storage.Init(driver.NewMemory()),
		KubeClient:   hookKubeClient,
		Capabilities: chartutil.DefaultCapabilities,
		Log: func(format string, v ...interface{}) {
			t.Helper()
			if *verbose {
				t.Logf(format, v...)
			}
		},
	}

	hookEvent := release.HookPreInstall

	r := &release.Release{
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
	}

	_ = configuration.execHook(r, hookEvent, 600)

	if !reflect.DeepEqual(hookKubeClient.deleteRecord, []resource.Info{
		{
			Name:      "build-config-1",
			Namespace: "test",
		},
		{
			Name:      "build-config-2",
			Namespace: "test",
		},
		{
			Name:      "build-config-2",
			Namespace: "test",
		},
	}) {
		t.Fatalf("Got unexpected delete record")
	}

	//if err != nil {
	//	t.Fatalf("An expected error occured: %#v", err)
	//}
}
