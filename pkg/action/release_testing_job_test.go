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
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"

	release "helm.sh/helm/v4/pkg/release/v1"
)

type releaseTestingClientGetter struct {
	RESTClientGetter
	config *rest.Config
}

func (g *releaseTestingClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.config, nil
}

func TestReleaseTestingGetPodLogs_Job(t *testing.T) {
	const (
		jobPath  = "/apis/batch/v1/namespaces/test-ns/jobs/smoke"
		podsPath = "/api/v1/namespaces/test-ns/pods"
	)
	tests := []struct {
		name          string
		kind          string
		event         release.HookEvent
		filters       map[string][]string
		selectorMode  string
		namespace     string
		manifest      string
		decoyJob      bool
		followingPod  bool
		noPods        bool
		failPath      string
		wantError     string
		wantPods      []string
		wantNoRequest bool
	}{
		{name: "generated selector", wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "manual selector", selectorMode: "manual", wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "included job", filters: map[string][]string{IncludeNameFilter: {"smoke"}}, wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "excluded job", filters: map[string][]string{ExcludeNameFilter: {"smoke"}}, wantNoRequest: true},
		{name: "unmatched include", filters: map[string][]string{IncludeNameFilter: {"other"}}, wantNoRequest: true},
		{name: "other hook event", event: release.HookPreInstall, wantNoRequest: true},
		{name: "non-pod resource", kind: "ConfigMap", wantNoRequest: true},
		{name: "pod hook", kind: "Pod", wantPods: []string{"smoke-alpha"}},
		{name: "job retrieval failure", failPath: jobPath, wantError: "unable to get job smoke"},
		{name: "pod list failure", failPath: podsPath, wantError: "unable to get pods for job smoke"},
		{name: "no pods", noPods: true, wantError: "no pods found for job smoke"},
		{name: "invalid selector", selectorMode: "invalid", wantError: "invalid pod selector for job smoke"},
		{name: "missing selector", selectorMode: "missing", wantError: "job smoke has no pod selector"},
		{name: "empty selector", selectorMode: "empty", wantError: "job smoke has no pod selector"},
		{name: "continue after pod log failure", failPath: podsPath + "/smoke-alpha/log", wantError: "unable to get logs for pod smoke-alpha", wantPods: []string{"smoke-zeta"}},
		{name: "manifest without namespace", manifest: "metadata:\n  name: smoke\n", wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "manifest in release namespace", namespace: "test-ns", wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "manifest in another namespace", namespace: "hook-ns", wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "ignore same-name job in release namespace", namespace: "hook-ns", decoyJob: true, wantPods: []string{"smoke-alpha", "smoke-zeta"}},
		{name: "later pod uses release namespace", namespace: "hook-ns", decoyJob: true, followingPod: true, wantPods: []string{"smoke-alpha", "smoke-zeta", "following"}},
		{name: "invalid job manifest", manifest: "metadata: [", wantError: "unable to parse metadata.namespace", wantNoRequest: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := &metav1.LabelSelector{MatchLabels: map[string]string{batchv1.ControllerUidLabel: "job-uid"}}
			wantSelector := "batch.kubernetes.io/controller-uid=job-uid"
			manual := tt.selectorMode == "manual"
			switch tt.selectorMode {
			case "manual":
				selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{"suite": "smoke"},
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"backend", "frontend"}},
					},
				}
				wantSelector = "suite=smoke,tier in (backend,frontend)"
			case "invalid":
				selector.MatchExpressions = []metav1.LabelSelectorRequirement{{Key: "tier", Operator: "Invalid"}}
			case "missing":
				selector = nil
			case "empty":
				selector = &metav1.LabelSelector{}
			}
			namespace := "test-ns"
			if tt.namespace != "" {
				namespace = tt.namespace
			}
			makePod := func(name, namespace string) v1.Pod {
				return v1.Pod{
					Name: name, Namespace: namespace,
					Spec: v1.PodSpec{
						InitContainers: []v1.Container{{Name: "setup"}},
						Containers:     []v1.Container{{Name: "test"}},
					},
				}
			}
			pods := []v1.Pod{
				makePod("smoke-zeta", namespace),
				makePod("smoke-alpha", namespace),
			}
			if tt.noPods {
				pods = nil
			}
			allPods := append([]v1.Pod(nil), pods...)
			hookPodsPath := "/api/v1/namespaces/" + namespace + "/pods"
			hookJobPath := "/apis/batch/v1/namespaces/" + namespace + "/jobs/smoke"
			objects := map[string]any{
				hookJobPath:  &batchv1.Job{Spec: batchv1.JobSpec{Selector: selector, ManualSelector: &manual}},
				hookPodsPath: &v1.PodList{Items: pods},
			}
			selectors := map[string]string{hookPodsPath: wantSelector}
			if tt.decoyJob {
				decoyPod := makePod("smoke-decoy", "test-ns")
				allPods = append(allPods, decoyPod)
				objects[jobPath] = &batchv1.Job{Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{batchv1.ControllerUidLabel: "decoy-uid"}},
				}}
				objects[podsPath] = &v1.PodList{Items: []v1.Pod{decoyPod}}
				selectors[podsPath] = "batch.kubernetes.io/controller-uid=decoy-uid"
			}
			if tt.followingPod {
				allPods = append(allPods, makePod("following", "test-ns"))
			}

			var requests []string
			httpClient := restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, http.MethodGet, req.Method)
				requests = append(requests, req.URL.Path)
				status := http.StatusOK
				object := objects[req.URL.Path]
				if selector, ok := selectors[req.URL.Path]; ok {
					assert.Equal(t, selector, req.URL.Query().Get("labelSelector"))
				}
				if object == nil {
					for _, pod := range allPods {
						podPath := "/api/v1/namespaces/" + pod.Namespace + "/pods/" + pod.Name
						if req.URL.Path == podPath {
							object = pod
						}
						if req.URL.Path == podPath+"/log" && req.URL.Path != tt.failPath {
							container := req.URL.Query().Get("container")
							assert.Contains(t, []string{"setup", "test"}, container)
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": {"text/plain"}},
								Body:       io.NopCloser(strings.NewReader(pod.Name + "/" + container + " logs\n")),
							}, nil
						}
					}
				}
				if req.URL.Path == tt.failPath {
					status = http.StatusForbidden
					object = &metav1.Status{Status: metav1.StatusFailure, Reason: metav1.StatusReasonForbidden, Code: http.StatusForbidden, Message: "access denied"}
				}
				if object == nil && req.URL.Path == jobPath && namespace == "hook-ns" {
					status = http.StatusNotFound
					object = &metav1.Status{Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound, Message: "not found"}
				}
				require.NotNil(t, object, "unexpected request: %s", req.URL)
				data, err := json.Marshal(object)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(data)),
				}, nil
			})
			config := &Configuration{RESTClientGetter: &releaseTestingClientGetter{
				config: &rest.Config{Host: "https://kubernetes.example", Transport: httpClient.Transport},
			}}
			client := NewReleaseTesting(config)
			client.Namespace = "test-ns"
			client.Filters = tt.filters
			hook := &release.Hook{Name: "smoke", Kind: "Job", Events: []release.HookEvent{release.HookTest}}
			hook.Manifest = tt.manifest
			if tt.namespace != "" {
				hook.Manifest = "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: smoke\n  namespace: " + tt.namespace + "\n"
			}
			if tt.kind != "" {
				hook.Kind = tt.kind
			}
			if tt.kind == "Pod" {
				hook.Name = "smoke-alpha"
			}
			if tt.event != "" {
				hook.Events = []release.HookEvent{tt.event}
			}
			hooks := []*release.Hook{hook}
			if tt.followingPod {
				hooks = append(hooks, &release.Hook{Name: "following", Kind: "Pod", Weight: 1, Events: []release.HookEvent{release.HookTest}})
			}
			var out bytes.Buffer
			err := client.GetPodLogs(&out, &release.Release{Hooks: hooks})
			assert.Equal(t, "test-ns", client.Namespace, "reading a hook's logs must not change the action namespace")
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				if tt.failPath != "" {
					assert.True(t, apierrors.IsForbidden(err), "the API error must remain identifiable")
				}
			} else {
				require.NoError(t, err)
			}
			if tt.wantNoRequest {
				assert.Empty(t, requests)
			}
			if tt.namespace == "hook-ns" {
				assert.Contains(t, requests, hookJobPath)
				assert.NotContains(t, requests, jobPath)
				assert.NotContains(t, out.String(), "smoke-decoy")
			}
			for _, pod := range tt.wantPods {
				for _, container := range []string{"setup", "test"} {
					assert.Contains(t, out.String(), "POD LOGS: "+pod+" ("+container+")\n"+pod+"/"+container+" logs\n")
				}
			}
			if len(tt.wantPods) == 0 {
				assert.Empty(t, out.String())
			}
			if len(tt.wantPods) > 1 {
				assert.Less(t, strings.Index(out.String(), tt.wantPods[0]), strings.Index(out.String(), tt.wantPods[1]), "pod output should be sorted by name")
			}
		})
	}
}
