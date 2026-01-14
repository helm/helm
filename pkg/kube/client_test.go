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

package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var (
	unstructuredSerializer = resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer
	codec                  = scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
)

func objBody(obj runtime.Object) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func newPod(name string) v1.Pod {
	return newPodWithStatus(name, v1.PodStatus{}, "")
}

func newPodWithStatus(name string, status v1.PodStatus, namespace string) v1.Pod {
	ns := v1.NamespaceDefault
	if namespace != "" {
		ns = namespace
	}
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			SelfLink:  "/api/v1/namespaces/default/pods/" + name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "app:v4",
				Image: "abc/app:v4",
				Ports: []v1.ContainerPort{{Name: "http", ContainerPort: 80}},
			}},
		},
		Status: status,
	}
}

func newPodList(names ...string) v1.PodList {
	var list v1.PodList
	for _, name := range names {
		list.Items = append(list.Items, newPod(name))
	}
	return list
}

func notFoundBody() *metav1.Status {
	return &metav1.Status{
		Code:    http.StatusNotFound,
		Status:  metav1.StatusFailure,
		Reason:  metav1.StatusReasonNotFound,
		Message: " \"\" not found",
		Details: &metav1.StatusDetails{},
	}
}

func newResponse(code int, obj runtime.Object) (*http.Response, error) {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	body := io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
	return &http.Response{StatusCode: code, Header: header, Body: body}, nil
}

func newResponseJSON(code int, json []byte) (*http.Response, error) {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	body := io.NopCloser(bytes.NewReader(json))
	return &http.Response{StatusCode: code, Header: header, Body: body}, nil
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	testFactory := cmdtesting.NewTestFactory()
	t.Cleanup(testFactory.Cleanup)

	return &Client{
		Factory: testFactory.WithNamespace(v1.NamespaceDefault),
	}
}

type RequestResponseAction struct {
	Request  http.Request
	Response http.Response
	Error    error
}

type RoundTripperTestFunc func(previous []RequestResponseAction, req *http.Request) (*http.Response, error)

func NewRequestResponseLogClient(t *testing.T, cb RoundTripperTestFunc) RequestResponseLogClient {
	t.Helper()
	return RequestResponseLogClient{
		t:  t,
		cb: cb,
	}
}

// RequestResponseLogClient is a test client that logs requests and responses
// Satisfying http.RoundTripper interface, it can be used to mock HTTP requests in tests.
// Forwarding requests to a callback function (cb) that can be used to simulate server responses.
type RequestResponseLogClient struct {
	t           *testing.T
	cb          RoundTripperTestFunc
	actionsLock sync.Mutex
	Actions     []RequestResponseAction
}

func (r *RequestResponseLogClient) Do(req *http.Request) (*http.Response, error) {
	t := r.t
	t.Helper()

	readBodyBytes := func(body io.ReadCloser) []byte {
		if body == nil {
			return []byte{}
		}

		defer body.Close()
		bodyBytes, err := io.ReadAll(body)
		require.NoError(t, err)

		return bodyBytes
	}

	reqBytes := readBodyBytes(req.Body)

	t.Logf("Request: %s %s %s", req.Method, req.URL.String(), reqBytes)
	if req.Body != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBytes))
	}

	resp, err := r.cb(r.Actions, req)

	respBytes := readBodyBytes(resp.Body)
	t.Logf("Response: %d %s", resp.StatusCode, string(respBytes))
	if resp.Body != nil {
		resp.Body = io.NopCloser(bytes.NewReader(respBytes))
	}

	r.actionsLock.Lock()
	defer r.actionsLock.Unlock()
	r.Actions = append(r.Actions, RequestResponseAction{
		Request:  *req,
		Response: *resp,
		Error:    err,
	})

	return resp, err
}

func TestCreate(t *testing.T) {
	// Note: c.Create with the fake client can currently only test creation of a single pod/object in the same list. When testing
	// with more than one pod, c.Create will run into a data race as it calls perform->batchPerform which performs creation
	// in batches. The race is something in the fake client itself in `func (c *RESTClient) do(...)`
	// when it stores the req: c.Req = req and cannot (?) be fixed easily.

	type testCase struct {
		Name                  string
		Pods                  v1.PodList
		Callback              func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error)
		ServerSideApply       bool
		ExpectedActions       []string
		ExpectedErrorContains string
	}

	testCases := map[string]testCase{
		"Create success (client-side apply)": {
			Pods:            newPodList("starfish"),
			ServerSideApply: false,
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, _ *http.Request) (*http.Response, error) {
				t.Helper()

				if len(previous) < 2 { // simulate a conflict
					return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
				}

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
			ExpectedActions: []string{
				"/namespaces/default/pods:POST",
				"/namespaces/default/pods:POST",
				"/namespaces/default/pods:POST",
			},
		},
		"Create success (server-side apply)": {
			Pods:            newPodList("whale"),
			ServerSideApply: true,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, _ *http.Request) (*http.Response, error) {
				t.Helper()

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
			ExpectedActions: []string{
				"/namespaces/default/pods/whale:PATCH",
			},
		},
		"Create fail: incompatible server (server-side apply)": {
			Pods:            newPodList("lobster"),
			ServerSideApply: true,
			Callback: func(t *testing.T, _ testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				return &http.Response{
					StatusCode: http.StatusUnsupportedMediaType,
					Request:    req,
				}, nil
			},
			ExpectedErrorContains: "server-side apply not available on the server:",
			ExpectedActions: []string{
				"/namespaces/default/pods/lobster:PATCH",
			},
		},
		"Create fail: quota (server-side apply)": {
			Pods:            newPodList("dolphin"),
			ServerSideApply: true,
			Callback: func(t *testing.T, _ testCase, _ []RequestResponseAction, _ *http.Request) (*http.Response, error) {
				t.Helper()

				return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
			},
			ExpectedErrorContains: "Operation cannot be fulfilled on resourcequotas \"quota\": the object has been modified; " +
				"please apply your changes to the latest version and try again",
			ExpectedActions: []string{
				"/namespaces/default/pods/dolphin:PATCH",
			},
		},
	}

	c := newTestClient(t)
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			client := NewRequestResponseLogClient(t, func(previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				return tc.Callback(t, tc, previous, req)
			})

			c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client:               fake.CreateHTTPClient(client.Do),
			}

			list, err := c.Build(objBody(&tc.Pods), false)
			require.NoError(t, err)
			if err != nil {
				t.Fatal(err)
			}

			result, err := c.Create(
				list,
				ClientCreateOptionServerSideApply(tc.ServerSideApply, false))
			if tc.ExpectedErrorContains != "" {
				require.ErrorContains(t, err, tc.ExpectedErrorContains)
			} else {
				require.NoError(t, err)

				// See note above about limitations in supporting more than a single object
				assert.Len(t, result.Created, 1, "expected 1 object created, got %d", len(result.Created))
			}

			actions := []string{}
			for _, action := range client.Actions {
				path, method := action.Request.URL.Path, action.Request.Method
				actions = append(actions, path+":"+method)
			}

			assert.Equal(t, tc.ExpectedActions, actions)

		})
	}
}

func TestUpdate(t *testing.T) {
	type testCase struct {
		OriginalPods                 v1.PodList
		TargetPods                   v1.PodList
		ThreeWayMergeForUnstructured bool
		ServerSideApply              bool
		ExpectedActions              []string
		ExpectedError                string
	}

	expectedActionsClientSideApply := []string{
		"/namespaces/default/pods/starfish:GET",
		"/namespaces/default/pods/starfish:GET",
		"/namespaces/default/pods/starfish:PATCH",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/dolphin:GET",
		"/namespaces/default/pods:POST", // create dolphin
		"/namespaces/default/pods:POST", // retry due to 409
		"/namespaces/default/pods:POST", // retry due to 409
		"/namespaces/default/pods/squid:GET",
		"/namespaces/default/pods/squid:DELETE",
		"/namespaces/default/pods/notfound:GET",
		"/namespaces/default/pods/notfound:DELETE",
	}

	expectedActionsServerSideApply := []string{
		"/namespaces/default/pods/starfish:GET",
		"/namespaces/default/pods/starfish:GET",
		"/namespaces/default/pods/starfish:PATCH",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:PATCH",
		"/namespaces/default/pods/dolphin:GET",
		"/namespaces/default/pods/dolphin:PATCH", // create dolphin
		"/namespaces/default/pods/squid:GET",
		"/namespaces/default/pods/squid:DELETE",
		"/namespaces/default/pods/notfound:GET",
		"/namespaces/default/pods/notfound:DELETE",
	}

	testCases := map[string]testCase{
		"client-side apply": {
			OriginalPods: newPodList("starfish", "otter", "squid", "notfound"),
			TargetPods: func() v1.PodList {
				listTarget := newPodList("starfish", "otter", "dolphin")
				listTarget.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return listTarget
			}(),
			ThreeWayMergeForUnstructured: false,
			ServerSideApply:              false,
			ExpectedActions:              expectedActionsClientSideApply,
			ExpectedError:                "",
		},
		"client-side apply (three-way merge for unstructured)": {
			OriginalPods: newPodList("starfish", "otter", "squid", "notfound"),
			TargetPods: func() v1.PodList {
				listTarget := newPodList("starfish", "otter", "dolphin")
				listTarget.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return listTarget
			}(),
			ThreeWayMergeForUnstructured: true,
			ServerSideApply:              false,
			ExpectedActions:              expectedActionsClientSideApply,
			ExpectedError:                "",
		},
		"serverSideApply": {
			OriginalPods: newPodList("starfish", "otter", "squid", "notfound"),
			TargetPods: func() v1.PodList {
				listTarget := newPodList("starfish", "otter", "dolphin")
				listTarget.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return listTarget
			}(),
			ThreeWayMergeForUnstructured: false,
			ServerSideApply:              true,
			ExpectedActions:              expectedActionsServerSideApply,
			ExpectedError:                "",
		},
		"serverSideApply with forbidden deletion": {
			OriginalPods: newPodList("starfish", "otter", "squid", "notfound", "forbidden"),
			TargetPods: func() v1.PodList {
				listTarget := newPodList("starfish", "otter", "dolphin")
				listTarget.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return listTarget
			}(),
			ThreeWayMergeForUnstructured: false,
			ServerSideApply:              true,
			ExpectedActions: append(expectedActionsServerSideApply,
				"/namespaces/default/pods/forbidden:GET",
				"/namespaces/default/pods/forbidden:DELETE",
			),
			ExpectedError: "failed to delete resource namespace=default, name=forbidden, kind=Pod:",
		},
		"rollback after failed upgrade with removed resource": {
			// Simulates rollback scenario:
			// - Revision 1 had "newpod"
			// - Revision 2 removed "newpod" but upgrade failed (OriginalPods is empty)
			// - Cluster still has "newpod" from Revision 1
			// - Rolling back to Revision 1 (TargetPods with "newpod") should succeed
			OriginalPods:                 v1.PodList{},         // Revision 2 (failed) - resource was removed
			TargetPods:                   newPodList("newpod"), // Revision 1 - rolling back to this
			ThreeWayMergeForUnstructured: false,
			ServerSideApply:              true,
			ExpectedActions: []string{
				"/namespaces/default/pods/newpod:GET",   // Check if resource exists
				"/namespaces/default/pods/newpod:GET",   // Get current state (first call in update path)
				"/namespaces/default/pods/newpod:GET",   // Get current cluster state to use as baseline
				"/namespaces/default/pods/newpod:PATCH", // Update using cluster state as baseline
			},
			ExpectedError: "",
		},
	}

	c := newTestClient(t)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			listOriginal := tc.OriginalPods
			listTarget := tc.TargetPods

			iterationCounter := 0
			cb := func(_ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				p, m := req.URL.Path, req.Method

				switch {
				case p == "/namespaces/default/pods/newpod" && m == http.MethodGet:
					return newResponse(http.StatusOK, &listTarget.Items[0])
				case p == "/namespaces/default/pods/newpod" && m == http.MethodPatch:
					return newResponse(http.StatusOK, &listTarget.Items[0])
				case p == "/namespaces/default/pods/starfish" && m == http.MethodGet:
					return newResponse(http.StatusOK, &listOriginal.Items[0])
				case p == "/namespaces/default/pods/otter" && m == http.MethodGet:
					return newResponse(http.StatusOK, &listOriginal.Items[1])
				case p == "/namespaces/default/pods/otter" && m == http.MethodPatch:
					if !tc.ServerSideApply {
						defer req.Body.Close()
						data, err := io.ReadAll(req.Body)
						require.NoError(t, err)

						assert.Equal(t, `{}`, string(data))
					}

					return newResponse(http.StatusOK, &listTarget.Items[0])
				case p == "/namespaces/default/pods/dolphin" && m == http.MethodGet:
					return newResponse(http.StatusNotFound, notFoundBody())
				case p == "/namespaces/default/pods/starfish" && m == http.MethodPatch:
					if !tc.ServerSideApply {
						// Ensure client-side apply specifies correct patch
						defer req.Body.Close()
						data, err := io.ReadAll(req.Body)
						require.NoError(t, err)

						expected := `{"spec":{"$setElementOrder/containers":[{"name":"app:v4"}],"containers":[{"$setElementOrder/ports":[{"containerPort":443}],"name":"app:v4","ports":[{"containerPort":443,"name":"https"},{"$patch":"delete","containerPort":80}]}]}}`
						assert.Equal(t, expected, string(data))
					}

					return newResponse(http.StatusOK, &listTarget.Items[0])
				case p == "/namespaces/default/pods" && m == http.MethodPost:
					if iterationCounter < 2 {
						iterationCounter++
						return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
					}

					return newResponse(http.StatusOK, &listTarget.Items[1])
				case p == "/namespaces/default/pods/dolphin" && m == http.MethodPatch:
					return newResponse(http.StatusOK, &listTarget.Items[1])
				case p == "/namespaces/default/pods/squid" && m == http.MethodDelete:
					return newResponse(http.StatusOK, &listTarget.Items[1])
				case p == "/namespaces/default/pods/squid" && m == http.MethodGet:
					return newResponse(http.StatusOK, &listTarget.Items[2])
				case p == "/namespaces/default/pods/notfound" && m == http.MethodGet:
					// Resource exists in original but will simulate not found on delete
					return newResponse(http.StatusOK, &listOriginal.Items[3])
				case p == "/namespaces/default/pods/notfound" && m == http.MethodDelete:
					// Simulate a not found during deletion; should not cause update to fail
					return newResponse(http.StatusNotFound, notFoundBody())
				case p == "/namespaces/default/pods/forbidden" && m == http.MethodGet:
					return newResponse(http.StatusOK, &listOriginal.Items[4])
				case p == "/namespaces/default/pods/forbidden" && m == http.MethodDelete:
					// Simulate RBAC forbidden that should cause update to fail
					return newResponse(http.StatusForbidden, &metav1.Status{
						Status:  metav1.StatusFailure,
						Message: "pods \"forbidden\" is forbidden: User \"test-user\" cannot delete resource \"pods\" in API group \"\" in the namespace \"default\"",
						Reason:  metav1.StatusReasonForbidden,
						Code:    http.StatusForbidden,
					})
				}

				t.FailNow()
				return nil, nil
			}

			client := NewRequestResponseLogClient(t, cb)

			c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client:               fake.CreateHTTPClient(client.Do),
			}

			first, err := c.Build(objBody(&listOriginal), false)
			require.NoError(t, err)

			second, err := c.Build(objBody(&listTarget), false)
			require.NoError(t, err)

			result, err := c.Update(
				first,
				second,
				ClientUpdateOptionThreeWayMergeForUnstructured(tc.ThreeWayMergeForUnstructured),
				ClientUpdateOptionForceReplace(false),
				ClientUpdateOptionServerSideApply(tc.ServerSideApply, false),
				ClientUpdateOptionUpgradeClientSideFieldManager(true))

			if tc.ExpectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.ExpectedError)
			} else {
				require.NoError(t, err)
			}

			// Special handling for the rollback test case
			if name == "rollback after failed upgrade with removed resource" {
				assert.Len(t, result.Created, 0, "expected 0 resource created, got %d", len(result.Created))
				assert.Len(t, result.Updated, 1, "expected 1 resource updated, got %d", len(result.Updated))
				assert.Len(t, result.Deleted, 0, "expected 0 resource deleted, got %d", len(result.Deleted))
			} else {
				assert.Len(t, result.Created, 1, "expected 1 resource created, got %d", len(result.Created))
				assert.Len(t, result.Updated, 2, "expected 2 resource updated, got %d", len(result.Updated))
				assert.Len(t, result.Deleted, 1, "expected 1 resource deleted, got %d", len(result.Deleted))
			}

			if tc.ExpectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.ExpectedError)
			} else {
				require.NoError(t, err)
			}

			actions := []string{}
			for _, action := range client.Actions {
				path, method := action.Request.URL.Path, action.Request.Method
				actions = append(actions, path+":"+method)
			}

			assert.Equal(t, tc.ExpectedActions, actions)
		})
	}
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		reader    io.Reader
		count     int
		err       bool
	}{
		{
			name:      "Valid input",
			namespace: "test",
			reader:    strings.NewReader(guestbookManifest),
			count:     6,
		}, {
			name:      "Valid input, deploying resources into different namespaces",
			namespace: "test",
			reader:    strings.NewReader(namespacedGuestbookManifest),
			count:     1,
		},
	}

	c := newTestClient(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test for an invalid manifest
			infos, err := c.Build(tt.reader, false)
			if err != nil && !tt.err {
				t.Errorf("Got error message when no error should have occurred: %v", err)
			} else if err != nil && strings.Contains(err.Error(), "--validate=false") {
				t.Error("error message was not scrubbed")
			}

			if len(infos) != tt.count {
				t.Errorf("expected %d result objects, got %d", tt.count, len(infos))
			}
		})
	}
}

func TestBuildTable(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		reader    io.Reader
		count     int
		err       bool
	}{
		{
			name:      "Valid input",
			namespace: "test",
			reader:    strings.NewReader(guestbookManifest),
			count:     6,
		}, {
			name:      "Valid input, deploying resources into different namespaces",
			namespace: "test",
			reader:    strings.NewReader(namespacedGuestbookManifest),
			count:     1,
		},
	}

	c := newTestClient(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test for an invalid manifest
			infos, err := c.BuildTable(tt.reader, false)
			if err != nil && !tt.err {
				t.Errorf("Got error message when no error should have occurred: %v", err)
			} else if err != nil && strings.Contains(err.Error(), "--validate=false") {
				t.Error("error message was not scrubbed")
			}

			if len(infos) != tt.count {
				t.Errorf("expected %d result objects, got %d", tt.count, len(infos))
			}
		})
	}
}

func TestPerform(t *testing.T) {
	tests := []struct {
		name       string
		reader     io.Reader
		count      int
		err        bool
		errMessage string
	}{
		{
			name:   "Valid input",
			reader: strings.NewReader(guestbookManifest),
			count:  6,
		}, {
			name:       "Empty manifests",
			reader:     strings.NewReader(""),
			err:        true,
			errMessage: "no objects visited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []*resource.Info{}

			fn := func(info *resource.Info) error {
				results = append(results, info)
				return nil
			}

			c := newTestClient(t)
			infos, err := c.Build(tt.reader, false)
			if err != nil && err.Error() != tt.errMessage {
				t.Errorf("Error while building manifests: %v", err)
			}

			err = perform(infos, fn)
			if (err != nil) != tt.err {
				t.Errorf("expected error: %v, got %v", tt.err, err)
			}
			if err != nil && err.Error() != tt.errMessage {
				t.Errorf("expected error message: %v, got %v", tt.errMessage, err)
			}

			if len(results) != tt.count {
				t.Errorf("expected %d result objects, got %d", tt.count, len(results))
			}
		})
	}
}

func TestWait(t *testing.T) {
	podList := newPodList("starfish", "otter", "squid")

	var created *time.Time

	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/api/v1/namespaces/default/pods/starfish" && m == http.MethodGet:
				pod := &podList.Items[0]
				if created != nil && time.Since(*created) >= time.Second*5 {
					pod.Status.Conditions = []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					}
				}
				return newResponse(http.StatusOK, pod)
			case p == "/api/v1/namespaces/default/pods/otter" && m == http.MethodGet:
				pod := &podList.Items[1]
				if created != nil && time.Since(*created) >= time.Second*5 {
					pod.Status.Conditions = []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					}
				}
				return newResponse(http.StatusOK, pod)
			case p == "/api/v1/namespaces/default/pods/squid" && m == http.MethodGet:
				pod := &podList.Items[2]
				if created != nil && time.Since(*created) >= time.Second*5 {
					pod.Status.Conditions = []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					}
				}
				return newResponse(http.StatusOK, pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				created = &now
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(&podList), false)
	if err != nil {
		t.Fatal(err)
	}

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 3 {
		t.Errorf("expected 3 resource created, got %d", len(result.Created))
	}

	if err := c.Wait(resources, time.Second*30); err != nil {
		t.Errorf("expected wait without error, got %s", err)
	}

	if time.Since(*created) < time.Second*5 {
		t.Errorf("expected to wait at least 5 seconds before ready status was detected, but got %s", time.Since(*created))
	}
}

func TestWaitJob(t *testing.T) {
	job := newJob("starfish", 0, intToInt32(1), 0, 0)

	var created *time.Time

	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/apis/batch/v1/namespaces/default/jobs/starfish" && m == http.MethodGet:
				if created != nil && time.Since(*created) >= time.Second*5 {
					job.Status.Succeeded = 1
				}
				return newResponse(http.StatusOK, job)
			case p == "/namespaces/default/jobs" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				created = &now
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(job), false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 1 {
		t.Errorf("expected 1 resource created, got %d", len(result.Created))
	}

	if err := c.WaitWithJobs(resources, time.Second*30); err != nil {
		t.Errorf("expected wait without error, got %s", err)
	}

	if time.Since(*created) < time.Second*5 {
		t.Errorf("expected to wait at least 5 seconds before ready status was detected, but got %s", time.Since(*created))
	}
}

func TestWaitDelete(t *testing.T) {
	pod := newPod("starfish")

	var deleted *time.Time

	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/starfish" && m == http.MethodGet:
				if deleted != nil && time.Since(*deleted) >= time.Second*5 {
					return newResponse(http.StatusNotFound, notFoundBody())
				}
				return newResponse(http.StatusOK, &pod)
			case p == "/namespaces/default/pods/starfish" && m == http.MethodDelete:
				now := time.Now()
				deleted = &now
				return newResponse(http.StatusOK, &pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(&pod), false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 1 {
		t.Errorf("expected 1 resource created, got %d", len(result.Created))
	}
	if _, err := c.Delete(resources, metav1.DeletePropagationBackground); err != nil {
		t.Fatal(err)
	}

	if err := c.WaitForDelete(resources, time.Second*30); err != nil {
		t.Errorf("expected wait without error, got %s", err)
	}

	if time.Since(*deleted) < time.Second*5 {
		t.Errorf("expected to wait at least 5 seconds before ready status was detected, but got %s", time.Since(*deleted))
	}
}

func TestReal(t *testing.T) {
	t.Skip("This is a live test, comment this line to run")
	c := New(nil)
	resources, err := c.Build(strings.NewReader(guestbookManifest), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Create(resources); err != nil {
		t.Fatal(err)
	}

	testSvcEndpointManifest := testServiceManifest + "\n---\n" + testEndpointManifest
	c = New(nil)
	resources, err = c.Build(strings.NewReader(testSvcEndpointManifest), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Create(resources); err != nil {
		t.Fatal(err)
	}

	resources, err = c.Build(strings.NewReader(testEndpointManifest), false)
	if err != nil {
		t.Fatal(err)
	}

	if _, errs := c.Delete(resources, metav1.DeletePropagationBackground); errs != nil {
		t.Fatal(errs)
	}

	resources, err = c.Build(strings.NewReader(testSvcEndpointManifest), false)
	if err != nil {
		t.Fatal(err)
	}
	// ensures that delete does not fail if a resource is not found
	if _, errs := c.Delete(resources, metav1.DeletePropagationBackground); errs != nil {
		t.Fatal(errs)
	}
}

func TestGetPodList(t *testing.T) {
	namespace := "some-namespace"
	names := []string{"dave", "jimmy"}
	var responsePodList v1.PodList
	for _, name := range names {
		responsePodList.Items = append(responsePodList.Items, newPodWithStatus(name, v1.PodStatus{}, namespace))
	}

	kubeClient := k8sfake.NewClientset(&responsePodList)
	c := Client{Namespace: namespace, kubeClient: kubeClient}

	podList, err := c.GetPodList(namespace, metav1.ListOptions{})
	clientAssertions := assert.New(t)
	clientAssertions.NoError(err)
	clientAssertions.Equal(&responsePodList, podList)
}

func TestOutputContainerLogsForPodList(t *testing.T) {
	namespace := "some-namespace"
	somePodList := newPodList("jimmy", "three", "structs")

	kubeClient := k8sfake.NewClientset(&somePodList)
	c := Client{Namespace: namespace, kubeClient: kubeClient}
	outBuffer := &bytes.Buffer{}
	outBufferFunc := func(_, _, _ string) io.Writer { return outBuffer }
	err := c.OutputContainerLogsForPodList(&somePodList, namespace, outBufferFunc)
	clientAssertions := assert.New(t)
	clientAssertions.NoError(err)
	clientAssertions.Equal("fake logsfake logsfake logs", outBuffer.String())
}

const testServiceManifest = `
kind: Service
apiVersion: v1
metadata:
  name: my-service
spec:
  selector:
    app: myapp
  ports:
    - port: 80
      protocol: TCP
      targetPort: 9376
`

const testEndpointManifest = `
kind: Endpoints
apiVersion: v1
metadata:
  name: my-service
subsets:
  - addresses:
      - ip: "1.2.3.4"
    ports:
      - port: 9376
`

const guestbookManifest = `
apiVersion: v1
kind: Service
metadata:
  name: redis-master
  labels:
    app: redis
    tier: backend
    role: master
spec:
  ports:
  - port: 6379
    targetPort: 6379
  selector:
    app: redis
    tier: backend
    role: master
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: redis-master
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: redis
        role: master
        tier: backend
    spec:
      containers:
      - name: master
        image: registry.k8s.io/redis:e2e  # or just image: redis
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        ports:
        - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: redis-replica
  labels:
    app: redis
    tier: backend
    role: replica
spec:
  ports:
    # the port that this service should serve on
  - port: 6379
  selector:
    app: redis
    tier: backend
    role: replica
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: redis-replica
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: redis
        role: replica
        tier: backend
    spec:
      containers:
      - name: replica
        image: gcr.io/google_samples/gb-redisreplica:v1
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        env:
        - name: GET_HOSTS_FROM
          value: dns
        ports:
        - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: frontend
  labels:
    app: guestbook
    tier: frontend
spec:
  ports:
  - port: 80
  selector:
    app: guestbook
    tier: frontend
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        env:
        - name: GET_HOSTS_FROM
          value: dns
        ports:
        - containerPort: 80
`

const namespacedGuestbookManifest = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: frontend
  namespace: guestbook
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        env:
        - name: GET_HOSTS_FROM
          value: dns
        ports:
        - containerPort: 80
`

var resourceQuotaConflict = []byte(`
{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"Operation cannot be fulfilled on resourcequotas \"quota\": the object has been modified; please apply your changes to the latest version and try again","reason":"Conflict","details":{"name":"quota","kind":"resourcequotas"},"code":409}`)

type createPatchTestCase struct {
	name string

	// The target state.
	target *unstructured.Unstructured
	// The state as it exists in the release.
	original *unstructured.Unstructured
	// The actual state as it exists in the cluster.
	actual *unstructured.Unstructured

	threeWayMergeForUnstructured bool
	// The patch is supposed to transfer the current state to the target state,
	// thereby preserving the actual state, wherever possible.
	expectedPatch     string
	expectedPatchType types.PatchType
}

func (c createPatchTestCase) run(t *testing.T) {
	scheme := runtime.NewScheme()
	v1.AddToScheme(scheme)
	encoder := jsonserializer.NewSerializerWithOptions(
		jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{
			Yaml: false, Pretty: false, Strict: true,
		},
	)
	objBody := func(obj runtime.Object) io.ReadCloser {
		return io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(encoder, obj))))
	}
	header := make(http.Header)
	header.Set("Content-Type", runtime.ContentTypeJSON)
	restClient := &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       objBody(c.actual),
			Header:     header,
		},
	}

	targetInfo := &resource.Info{
		Client:    restClient,
		Namespace: "default",
		Name:      "test-obj",
		Object:    c.target,
		Mapping: &meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    "crd.com",
				Version:  "v1",
				Resource: "datas",
			},
			Scope: meta.RESTScopeNamespace,
		},
	}

	patch, patchType, err := createPatch(c.original, targetInfo, c.threeWayMergeForUnstructured)
	if err != nil {
		t.Fatalf("Failed to create patch: %v", err)
	}

	if c.expectedPatch != string(patch) {
		t.Errorf("Unexpected patch.\nTarget:\n%s\nOriginal:\n%s\nActual:\n%s\n\nExpected:\n%s\nGot:\n%s",
			c.target,
			c.original,
			c.actual,
			c.expectedPatch,
			string(patch),
		)
	}

	if patchType != types.MergePatchType {
		t.Errorf("Expected patch type %s, got %s", types.MergePatchType, patchType)
	}
}

func newTestCustomResourceData(metadata map[string]string, spec map[string]interface{}) *unstructured.Unstructured {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	if _, ok := metadata["name"]; !ok {
		metadata["name"] = "test-obj"
	}
	if _, ok := metadata["namespace"]; !ok {
		metadata["namespace"] = "default"
	}
	o := map[string]interface{}{
		"apiVersion": "crd.com/v1",
		"kind":       "Data",
		"metadata":   metadata,
	}
	if len(spec) > 0 {
		o["spec"] = spec
	}
	return &unstructured.Unstructured{
		Object: o,
	}
}

func TestCreatePatchCustomResourceMetadata(t *testing.T) {
	target := newTestCustomResourceData(map[string]string{
		"meta.helm.sh/release-name":      "foo-simple",
		"meta.helm.sh/release-namespace": "default",
		"objectset.rio.cattle.io/id":     "default-foo-simple",
	}, nil)
	testCase := createPatchTestCase{
		name:     "take ownership of resource",
		target:   target,
		original: target,
		actual: newTestCustomResourceData(nil, map[string]interface{}{
			"color": "red",
		}),
		threeWayMergeForUnstructured: true,
		expectedPatch:                `{"metadata":{"meta.helm.sh/release-name":"foo-simple","meta.helm.sh/release-namespace":"default","objectset.rio.cattle.io/id":"default-foo-simple"}}`,
		expectedPatchType:            types.MergePatchType,
	}
	t.Run(testCase.name, testCase.run)

	// Previous behavior.
	testCase.threeWayMergeForUnstructured = false
	testCase.expectedPatch = `{}`
	t.Run(testCase.name, testCase.run)
}

func TestCreatePatchCustomResourceSpec(t *testing.T) {
	target := newTestCustomResourceData(nil, map[string]interface{}{
		"color": "red",
		"size":  "large",
	})
	testCase := createPatchTestCase{
		name:     "merge with spec of existing custom resource",
		target:   target,
		original: target,
		actual: newTestCustomResourceData(nil, map[string]interface{}{
			"color":  "red",
			"weight": "heavy",
		}),
		threeWayMergeForUnstructured: true,
		expectedPatch:                `{"spec":{"size":"large"}}`,
		expectedPatchType:            types.MergePatchType,
	}
	t.Run(testCase.name, testCase.run)

	// Previous behavior.
	testCase.threeWayMergeForUnstructured = false
	testCase.expectedPatch = `{}`
	t.Run(testCase.name, testCase.run)
}

type errorFactory struct {
	*cmdtesting.TestFactory
	err error
}

func (f *errorFactory) KubernetesClientSet() (*kubernetes.Clientset, error) {
	return nil, f.err
}

func newTestClientWithDiscoveryError(t *testing.T, err error) *Client {
	t.Helper()
	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/version" {
				return nil, err
			}
			resp, respErr := newResponse(http.StatusOK, &v1.Pod{})
			return resp, respErr
		}),
	}
	return c
}

func TestIsReachable(t *testing.T) {
	const (
		expectedUnreachableMsg = "kubernetes cluster unreachable"
	)
	tests := []struct {
		name          string
		setupClient   func(*testing.T) *Client
		expectError   bool
		errorContains string
	}{
		{
			name: "successful reachability test",
			setupClient: func(t *testing.T) *Client {
				t.Helper()
				client := newTestClient(t)
				client.kubeClient = k8sfake.NewClientset()
				return client
			},
			expectError: false,
		},
		{
			name: "client creation error with ErrEmptyConfig",
			setupClient: func(t *testing.T) *Client {
				t.Helper()
				client := newTestClient(t)
				client.Factory = &errorFactory{err: genericclioptions.ErrEmptyConfig}
				return client
			},
			expectError:   true,
			errorContains: expectedUnreachableMsg,
		},
		{
			name: "client creation error with general error",
			setupClient: func(t *testing.T) *Client {
				t.Helper()
				client := newTestClient(t)
				client.Factory = &errorFactory{err: errors.New("connection refused")}
				return client
			},
			expectError:   true,
			errorContains: "kubernetes cluster unreachable: connection refused",
		},
		{
			name: "discovery error with cluster unreachable",
			setupClient: func(t *testing.T) *Client {
				t.Helper()
				return newTestClientWithDiscoveryError(t, http.ErrServerClosed)
			},
			expectError:   true,
			errorContains: expectedUnreachableMsg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient(t)
			err := client.IsReachable()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}

				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error message to contain '%s', got: %v", tt.errorContains, err)
				}

			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestIsIncompatibleServerError(t *testing.T) {
	testCases := map[string]struct {
		Err  error
		Want bool
	}{
		"Unsupported media type": {
			Err:  &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusUnsupportedMediaType}},
			Want: true,
		},
		"Not found error": {
			Err:  &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusNotFound}},
			Want: false,
		},
		"Generic error": {
			Err:  fmt.Errorf("some generic error"),
			Want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := isIncompatibleServerError(tc.Err); got != tc.Want {
				t.Errorf("isIncompatibleServerError() = %v, want %v", got, tc.Want)
			}
		})
	}
}

func TestReplaceResource(t *testing.T) {
	type testCase struct {
		Pods                  v1.PodList
		Callback              func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error)
		ExpectedErrorContains string
	}

	testCases := map[string]testCase{
		"normal": {
			Pods: newPodList("whale"),
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				switch len(previous) {
				case 0:
					assert.Equal(t, "GET", req.Method)
				case 1:
					assert.Equal(t, "PUT", req.Method)
				}

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"conflict": {
			Pods: newPodList("whale"),
			Callback: func(t *testing.T, _ testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				return &http.Response{
					StatusCode: http.StatusConflict,
					Request:    req,
				}, nil
			},
			ExpectedErrorContains: "failed to replace object: the server reported a conflict",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			testFactory := cmdtesting.NewTestFactory()
			t.Cleanup(testFactory.Cleanup)

			client := NewRequestResponseLogClient(t, func(previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				return tc.Callback(t, tc, previous, req)
			})

			testFactory.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client:               fake.CreateHTTPClient(client.Do),
			}

			resourceList, err := buildResourceList(testFactory, v1.NamespaceDefault, FieldValidationDirectiveStrict, objBody(&tc.Pods), nil)
			require.NoError(t, err)

			require.Len(t, resourceList, 1)
			info := resourceList[0]

			err = replaceResource(info, FieldValidationDirectiveStrict)
			if tc.ExpectedErrorContains != "" {
				require.ErrorContains(t, err, tc.ExpectedErrorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, info.Object)
			}
		})
	}
}

func TestPatchResourceClientSide(t *testing.T) {
	type testCase struct {
		OriginalPods                 v1.PodList
		TargetPods                   v1.PodList
		ThreeWayMergeForUnstructured bool
		Callback                     func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error)
		ExpectedErrorContains        string
	}

	testCases := map[string]testCase{
		"normal": {
			OriginalPods: newPodList("whale"),
			TargetPods: func() v1.PodList {
				pods := newPodList("whale")
				pods.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return pods
			}(),
			ThreeWayMergeForUnstructured: false,
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				switch len(previous) {
				case 0:
					assert.Equal(t, "GET", req.Method)
					return newResponse(http.StatusOK, &tc.OriginalPods.Items[0])
				case 1:
					assert.Equal(t, "PATCH", req.Method)
					assert.Equal(t, "application/strategic-merge-patch+json", req.Header.Get("Content-Type"))
					return newResponse(http.StatusOK, &tc.TargetPods.Items[0])
				}

				t.Fail()
				return nil, nil
			},
		},
		"three way merge for unstructured": {
			OriginalPods: newPodList("whale"),
			TargetPods: func() v1.PodList {
				pods := newPodList("whale")
				pods.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return pods
			}(),
			ThreeWayMergeForUnstructured: true,
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				switch len(previous) {
				case 0:
					assert.Equal(t, "GET", req.Method)
					return newResponse(http.StatusOK, &tc.OriginalPods.Items[0])
				case 1:
					t.Logf("patcher: %+v", req.Header)
					assert.Equal(t, "PATCH", req.Method)
					assert.Equal(t, "application/strategic-merge-patch+json", req.Header.Get("Content-Type"))
					return newResponse(http.StatusOK, &tc.TargetPods.Items[0])
				}

				t.Fail()
				return nil, nil
			},
		},
		"conflict": {
			OriginalPods: newPodList("whale"),
			TargetPods: func() v1.PodList {
				pods := newPodList("whale")
				pods.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

				return pods
			}(),
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				switch len(previous) {
				case 0:
					assert.Equal(t, "GET", req.Method)
					return newResponse(http.StatusOK, &tc.OriginalPods.Items[0])
				case 1:
					assert.Equal(t, "PATCH", req.Method)
					return &http.Response{
						StatusCode: http.StatusConflict,
						Request:    req,
					}, nil
				}

				t.Fail()
				return nil, nil

			},
			ExpectedErrorContains: "cannot patch \"whale\" with kind Pod: the server reported a conflict",
		},
		"no patch": {
			OriginalPods: newPodList("whale"),
			TargetPods:   newPodList("whale"),
			Callback: func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				switch len(previous) {
				case 0:
					assert.Equal(t, "GET", req.Method)
					return newResponse(http.StatusOK, &tc.OriginalPods.Items[0])
				case 1:
					assert.Equal(t, "GET", req.Method)
					return newResponse(http.StatusOK, &tc.TargetPods.Items[0])
				}

				t.Fail()
				return nil, nil // newResponse(http.StatusOK, &tc.TargetPods.Items[0])

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			testFactory := cmdtesting.NewTestFactory()
			t.Cleanup(testFactory.Cleanup)

			client := NewRequestResponseLogClient(t, func(previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				return tc.Callback(t, tc, previous, req)
			})

			testFactory.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client:               fake.CreateHTTPClient(client.Do),
			}

			resourceListOriginal, err := buildResourceList(testFactory, v1.NamespaceDefault, FieldValidationDirectiveStrict, objBody(&tc.OriginalPods), nil)
			require.NoError(t, err)
			require.Len(t, resourceListOriginal, 1)

			resourceListTarget, err := buildResourceList(testFactory, v1.NamespaceDefault, FieldValidationDirectiveStrict, objBody(&tc.TargetPods), nil)
			require.NoError(t, err)
			require.Len(t, resourceListTarget, 1)

			original := resourceListOriginal[0]
			target := resourceListTarget[0]

			err = patchResourceClientSide(original.Object, target, tc.ThreeWayMergeForUnstructured)
			if tc.ExpectedErrorContains != "" {
				require.ErrorContains(t, err, tc.ExpectedErrorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, target.Object)
			}
		})
	}
}

func TestPatchResourceServerSide(t *testing.T) {
	type testCase struct {
		Pods                     v1.PodList
		DryRun                   bool
		ForceConflicts           bool
		FieldValidationDirective FieldValidationDirective
		Callback                 func(t *testing.T, tc testCase, previous []RequestResponseAction, req *http.Request) (*http.Response, error)
		ExpectedErrorContains    string
	}

	testCases := map[string]testCase{
		"normal": {
			Pods:                     newPodList("whale"),
			DryRun:                   false,
			ForceConflicts:           false,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "PATCH", req.Method)
				assert.Equal(t, "application/apply-patch+yaml", req.Header.Get("Content-Type"))
				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				assert.Equal(t, "false", req.URL.Query().Get("force"))
				assert.Equal(t, "Strict", req.URL.Query().Get("fieldValidation"))

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"dry run": {
			Pods:                     newPodList("whale"),
			DryRun:                   true,
			ForceConflicts:           false,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "PATCH", req.Method)
				assert.Equal(t, "application/apply-patch+yaml", req.Header.Get("Content-Type"))
				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				assert.Equal(t, "All", req.URL.Query().Get("dryRun"))
				assert.Equal(t, "false", req.URL.Query().Get("force"))
				assert.Equal(t, "Strict", req.URL.Query().Get("fieldValidation"))

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"force conflicts": {
			Pods:                     newPodList("whale"),
			DryRun:                   false,
			ForceConflicts:           true,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "PATCH", req.Method)
				assert.Equal(t, "application/apply-patch+yaml", req.Header.Get("Content-Type"))
				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				assert.Equal(t, "true", req.URL.Query().Get("force"))
				assert.Equal(t, "Strict", req.URL.Query().Get("fieldValidation"))

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"dry run + force conflicts": {
			Pods:                     newPodList("whale"),
			DryRun:                   true,
			ForceConflicts:           true,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "PATCH", req.Method)
				assert.Equal(t, "application/apply-patch+yaml", req.Header.Get("Content-Type"))
				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				assert.Equal(t, "All", req.URL.Query().Get("dryRun"))
				assert.Equal(t, "true", req.URL.Query().Get("force"))
				assert.Equal(t, "Strict", req.URL.Query().Get("fieldValidation"))

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"field validation ignore": {
			Pods:                     newPodList("whale"),
			DryRun:                   false,
			ForceConflicts:           false,
			FieldValidationDirective: FieldValidationDirectiveIgnore,
			Callback: func(t *testing.T, tc testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				assert.Equal(t, "PATCH", req.Method)
				assert.Equal(t, "application/apply-patch+yaml", req.Header.Get("Content-Type"))
				assert.Equal(t, "/namespaces/default/pods/whale", req.URL.Path)
				assert.Equal(t, "false", req.URL.Query().Get("force"))
				assert.Equal(t, "Ignore", req.URL.Query().Get("fieldValidation"))

				return newResponse(http.StatusOK, &tc.Pods.Items[0])
			},
		},
		"incompatible server": {
			Pods:                     newPodList("whale"),
			DryRun:                   false,
			ForceConflicts:           false,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, _ testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				return &http.Response{
					StatusCode: http.StatusUnsupportedMediaType,
					Request:    req,
				}, nil
			},
			ExpectedErrorContains: "server-side apply not available on the server:",
		},
		"conflict": {
			Pods:                     newPodList("whale"),
			DryRun:                   false,
			ForceConflicts:           false,
			FieldValidationDirective: FieldValidationDirectiveStrict,
			Callback: func(t *testing.T, _ testCase, _ []RequestResponseAction, req *http.Request) (*http.Response, error) {
				t.Helper()

				return &http.Response{
					StatusCode: http.StatusConflict,
					Request:    req,
				}, nil
			},
			ExpectedErrorContains: "the server reported a conflict",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			testFactory := cmdtesting.NewTestFactory()
			t.Cleanup(testFactory.Cleanup)

			client := NewRequestResponseLogClient(t, func(previous []RequestResponseAction, req *http.Request) (*http.Response, error) {
				return tc.Callback(t, tc, previous, req)
			})

			testFactory.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client:               fake.CreateHTTPClient(client.Do),
			}

			resourceList, err := buildResourceList(testFactory, v1.NamespaceDefault, tc.FieldValidationDirective, objBody(&tc.Pods), nil)
			require.NoError(t, err)

			require.Len(t, resourceList, 1)
			info := resourceList[0]

			err = patchResourceServerSide(info, tc.DryRun, tc.ForceConflicts, tc.FieldValidationDirective)
			if tc.ExpectedErrorContains != "" {
				require.ErrorContains(t, err, tc.ExpectedErrorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, info.Object)
			}
		})
	}
}

func TestDetermineFieldValidationDirective(t *testing.T) {

	assert.Equal(t, FieldValidationDirectiveIgnore, determineFieldValidationDirective(false))
	assert.Equal(t, FieldValidationDirectiveStrict, determineFieldValidationDirective(true))
}

func TestClientWaitContextCancellationLegacy(t *testing.T) {
	podList := newPodList("starfish", "otter")

	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	requestCount := 0
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			requestCount++
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)

			if requestCount == 2 {
				cancel()
			}

			switch {
			case p == "/api/v1/namespaces/default/pods/starfish" && m == http.MethodGet:
				pod := &podList.Items[0]
				pod.Status.Conditions = []v1.PodCondition{
					{
						Type:   v1.PodReady,
						Status: v1.ConditionFalse,
					},
				}
				return newResponse(http.StatusOK, pod)
			case p == "/api/v1/namespaces/default/pods/otter" && m == http.MethodGet:
				pod := &podList.Items[1]
				pod.Status.Conditions = []v1.PodCondition{
					{
						Type:   v1.PodReady,
						Status: v1.ConditionFalse,
					},
				}
				return newResponse(http.StatusOK, pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Logf("unexpected request: %s %s", req.Method, req.URL.Path)
				return newResponse(http.StatusNotFound, notFoundBody())
			}
		}),
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	require.NoError(t, err)

	resources, err := c.Build(objBody(&podList), false)
	require.NoError(t, err)

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	require.NoError(t, err)
	assert.Len(t, result.Created, 2, "expected 2 resources created, got %d", len(result.Created))

	err = c.Wait(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitWithJobsContextCancellationLegacy(t *testing.T) {
	job := newJob("starfish", 0, intToInt32(1), 0, 0)

	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	requestCount := 0
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			requestCount++
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)

			if requestCount == 2 {
				cancel()
			}

			switch {
			case p == "/apis/batch/v1/namespaces/default/jobs/starfish" && m == http.MethodGet:
				job.Status.Succeeded = 0
				return newResponse(http.StatusOK, job)
			case p == "/namespaces/default/jobs" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Logf("unexpected request: %s %s", req.Method, req.URL.Path)
				return newResponse(http.StatusNotFound, notFoundBody())
			}
		}),
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	require.NoError(t, err)

	resources, err := c.Build(objBody(job), false)
	require.NoError(t, err)

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	require.NoError(t, err)
	assert.Len(t, result.Created, 1, "expected 1 resource created, got %d", len(result.Created))

	err = c.WaitWithJobs(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitForDeleteContextCancellationLegacy(t *testing.T) {
	pod := newPod("starfish")

	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	deleted := false
	requestCount := 0
	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			requestCount++
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)

			if requestCount == 3 {
				cancel()
			}

			switch {
			case p == "/namespaces/default/pods/starfish" && m == http.MethodGet:
				if deleted {
					return newResponse(http.StatusOK, &pod)
				}
				return newResponse(http.StatusOK, &pod)
			case p == "/namespaces/default/pods/starfish" && m == http.MethodDelete:
				deleted = true
				return newResponse(http.StatusOK, &pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Logf("unexpected request: %s %s", req.Method, req.URL.Path)
				return newResponse(http.StatusNotFound, notFoundBody())
			}
		}),
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	require.NoError(t, err)

	resources, err := c.Build(objBody(&pod), false)
	require.NoError(t, err)

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	require.NoError(t, err)
	assert.Len(t, result.Created, 1, "expected 1 resource created, got %d", len(result.Created))

	if _, err := c.Delete(resources, metav1.DeletePropagationBackground); err != nil {
		t.Fatal(err)
	}

	err = c.WaitForDelete(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitContextNilDoesNotPanic(t *testing.T) {
	podList := newPodList("starfish")

	var created *time.Time

	c := newTestClient(t)
	c.WaitContext = nil

	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/api/v1/namespaces/default/pods/starfish" && m == http.MethodGet:
				pod := &podList.Items[0]
				if created != nil && time.Since(*created) >= time.Second*2 {
					pod.Status.Conditions = []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					}
				}
				return newResponse(http.StatusOK, pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				created = &now
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	require.NoError(t, err)

	resources, err := c.Build(objBody(&podList), false)
	require.NoError(t, err)

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	require.NoError(t, err)
	assert.Len(t, result.Created, 1, "expected 1 resource created, got %d", len(result.Created))

	err = c.Wait(resources, time.Second*30)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, time.Since(*created), time.Second*2, "expected to wait at least 2 seconds")
}

func TestClientWaitContextPreCancelledLegacy(t *testing.T) {
	podList := newPodList("starfish")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c := newTestClient(t)
	c.WaitContext = ctx

	c.Factory.(*cmdtesting.TestFactory).Client = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/api/v1/namespaces/default/pods/starfish" && m == http.MethodGet:
				pod := &podList.Items[0]
				return newResponse(http.StatusOK, pod)
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				resources, err := c.Build(req.Body, false)
				if err != nil {
					t.Fatal(err)
				}
				return newResponse(http.StatusOK, resources[0].Object)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(LegacyStrategy)
	require.NoError(t, err)

	resources, err := c.Build(objBody(&podList), false)
	require.NoError(t, err)

	result, err := c.Create(
		resources,
		ClientCreateOptionServerSideApply(false, false))
	require.NoError(t, err)
	assert.Len(t, result.Created, 1, "expected 1 resource created, got %d", len(result.Created))

	err = c.Wait(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitContextCancellationStatusWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	podManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
`
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(StatusWatcherStrategy)
	require.NoError(t, err)

	resources, err := c.Build(strings.NewReader(podManifest), false)
	require.NoError(t, err)

	cancel()

	err = c.Wait(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitWithJobsContextCancellationStatusWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	jobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: test-job
  namespace: default
`
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(StatusWatcherStrategy)
	require.NoError(t, err)

	resources, err := c.Build(strings.NewReader(jobManifest), false)
	require.NoError(t, err)

	cancel()

	err = c.WaitWithJobs(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

func TestClientWaitForDeleteContextCancellationStatusWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	c := newTestClient(t)
	c.WaitContext = ctx

	podManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(StatusWatcherStrategy)
	require.NoError(t, err)

	resources, err := c.Build(strings.NewReader(podManifest), false)
	require.NoError(t, err)

	cancel()

	err = c.WaitForDelete(resources, time.Second*30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled", "expected context canceled error, got: %v", err)
}

// testStatusReader is a custom status reader for testing that returns a configurable status.
type testStatusReader struct {
	supportedGK schema.GroupKind
	status      status.Status
}

func (r *testStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == r.supportedGK
}

func (r *testStatusReader) ReadStatus(_ context.Context, _ engine.ClusterReader, id object.ObjMetadata) (*event.ResourceStatus, error) {
	return &event.ResourceStatus{
		Identifier: id,
		Status:     r.status,
		Message:    "test status reader",
	}, nil
}

func (r *testStatusReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader, u *unstructured.Unstructured) (*event.ResourceStatus, error) {
	id := object.ObjMetadata{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
		GroupKind: u.GroupVersionKind().GroupKind(),
	}
	return &event.ResourceStatus{
		Identifier: id,
		Status:     r.status,
		Message:    "test status reader",
	}, nil
}

func TestClientStatusReadersPassedToStatusWaiter(t *testing.T) {
	// This test verifies that Client.StatusReaders is correctly passed through
	// to the statusWaiter when using the StatusWatcherStrategy.
	// We use a custom status reader that immediately returns CurrentStatus for pods,
	// which allows a pod without Ready condition to pass the wait.
	podManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
`

	c := newTestClient(t)
	statusReaders := []engine.StatusReader{
		&testStatusReader{
			supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
			status:      status.CurrentStatus,
		},
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(StatusWatcherStrategy, WithKStatusReaders(statusReaders...))
	require.NoError(t, err)

	resources, err := c.Build(strings.NewReader(podManifest), false)
	require.NoError(t, err)

	// The pod has no Ready condition, but our custom reader returns CurrentStatus,
	// so the wait should succeed immediately without timeout.
	err = c.Wait(resources, time.Second*3)
	require.NoError(t, err)
}

func TestClientStatusReadersWithWaitWithJobs(t *testing.T) {
	// This test verifies that Client.StatusReaders is correctly passed through
	// to the statusWaiter when using WaitWithJobs.
	jobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: test-job
  namespace: default
`

	c := newTestClient(t)
	statusReaders := []engine.StatusReader{
		&testStatusReader{
			supportedGK: schema.GroupKind{Group: "batch", Kind: "Job"},
			status:      status.CurrentStatus,
		},
	}

	var err error
	c.Waiter, err = c.GetWaiterWithOptions(StatusWatcherStrategy, WithKStatusReaders(statusReaders...))
	require.NoError(t, err)

	resources, err := c.Build(strings.NewReader(jobManifest), false)
	require.NoError(t, err)

	// The job has no Complete condition, but our custom reader returns CurrentStatus,
	// so the wait should succeed immediately without timeout.
	err = c.WaitWithJobs(resources, time.Second*3)
	require.NoError(t, err)
}
