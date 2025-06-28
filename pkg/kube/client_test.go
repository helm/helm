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
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
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
		Factory: testFactory.WithNamespace("default"),
	}
}

func TestCreate(t *testing.T) {
	// Note: c.Create with the fake client can currently only test creation of a single pod in the same list. When testing
	// with more than one pod, c.Create will run into a data race as it calls perform->batchPerform which performs creation
	// in batches. The first data race is on accessing var actions and can be fixed easily with a mutex lock in the Client
	// function. The second data race though is something in the fake client itself in  func (c *RESTClient) do(...)
	// when it stores the req: c.Req = req and cannot (?) be fixed easily.
	listA := newPodList("starfish")
	listB := newPodList("dolphin")

	var actions []string
	var iterationCounter int

	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			path, method := req.URL.Path, req.Method
			bodyReader := new(strings.Builder)
			_, _ = io.Copy(bodyReader, req.Body)
			body := bodyReader.String()
			actions = append(actions, path+":"+method)
			t.Logf("got request %s %s", path, method)
			switch {
			case path == "/namespaces/default/pods" && method == http.MethodPost:
				if strings.Contains(body, "starfish") {
					if iterationCounter < 2 {
						iterationCounter++
						return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
					}
					return newResponse(http.StatusOK, &listA.Items[0])
				}
				return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
			default:
				t.Fatalf("unexpected request: %s %s", method, path)
				return nil, nil
			}
		}),
	}

	t.Run("Create success", func(t *testing.T) {
		list, err := c.Build(objBody(&listA), false)
		if err != nil {
			t.Fatal(err)
		}

		result, err := c.Create(list)
		if err != nil {
			t.Fatal(err)
		}

		if len(result.Created) != 1 {
			t.Errorf("expected 1 resource created, got %d", len(result.Created))
		}

		expectedActions := []string{
			"/namespaces/default/pods:POST",
			"/namespaces/default/pods:POST",
			"/namespaces/default/pods:POST",
		}
		if len(expectedActions) != len(actions) {
			t.Fatalf("unexpected number of requests, expected %d, got %d", len(expectedActions), len(actions))
		}
		for k, v := range expectedActions {
			if actions[k] != v {
				t.Errorf("expected %s request got %s", v, actions[k])
			}
		}
	})

	t.Run("Create failure", func(t *testing.T) {
		list, err := c.Build(objBody(&listB), false)
		if err != nil {
			t.Fatal(err)
		}

		_, err = c.Create(list)
		if err == nil {
			t.Errorf("expected error")
		}

		expectedString := "Operation cannot be fulfilled on resourcequotas \"quota\": the object has been modified; " +
			"please apply your changes to the latest version and try again"
		if !strings.Contains(err.Error(), expectedString) {
			t.Errorf("Unexpected error message: %q", err)
		}

		expectedActions := []string{
			"/namespaces/default/pods:POST",
		}
		for k, v := range actions {
			if expectedActions[0] != v {
				t.Errorf("expected %s request got %s", v, actions[k])
			}
		}
	})
}

func testUpdate(t *testing.T, threeWayMerge bool) {
	t.Helper()
	listA := newPodList("starfish", "otter", "squid")
	listB := newPodList("starfish", "otter", "dolphin")
	listC := newPodList("starfish", "otter", "dolphin")
	listB.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}
	listC.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

	var actions []string
	var iterationCounter int

	c := newTestClient(t)
	c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			actions = append(actions, p+":"+m)
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/starfish" && m == http.MethodGet:
				return newResponse(http.StatusOK, &listA.Items[0])
			case p == "/namespaces/default/pods/otter" && m == http.MethodGet:
				return newResponse(http.StatusOK, &listA.Items[1])
			case p == "/namespaces/default/pods/otter" && m == http.MethodPatch:
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("could not dump request: %s", err)
				}
				req.Body.Close()
				expected := `{}`
				if string(data) != expected {
					t.Errorf("expected patch\n%s\ngot\n%s", expected, string(data))
				}
				return newResponse(http.StatusOK, &listB.Items[0])
			case p == "/namespaces/default/pods/dolphin" && m == http.MethodGet:
				return newResponse(http.StatusNotFound, notFoundBody())
			case p == "/namespaces/default/pods/starfish" && m == http.MethodPatch:
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("could not dump request: %s", err)
				}
				req.Body.Close()
				expected := `{"spec":{"$setElementOrder/containers":[{"name":"app:v4"}],"containers":[{"$setElementOrder/ports":[{"containerPort":443}],"name":"app:v4","ports":[{"containerPort":443,"name":"https"},{"$patch":"delete","containerPort":80}]}]}}`
				if string(data) != expected {
					t.Errorf("expected patch\n%s\ngot\n%s", expected, string(data))
				}
				return newResponse(http.StatusOK, &listB.Items[0])
			case p == "/namespaces/default/pods" && m == http.MethodPost:
				if iterationCounter < 2 {
					iterationCounter++
					return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
				}
				return newResponse(http.StatusOK, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == http.MethodDelete:
				return newResponse(http.StatusOK, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == http.MethodGet:
				return newResponse(http.StatusOK, &listB.Items[2])
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}
	first, err := c.Build(objBody(&listA), false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Build(objBody(&listB), false)
	if err != nil {
		t.Fatal(err)
	}

	result, err := c.Update(first, second, false, threeWayMerge)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Created) != 1 {
		t.Errorf("expected 1 resource created, got %d", len(result.Created))
	}
	if len(result.Updated) != 2 {
		t.Errorf("expected 2 resource updated, got %d", len(result.Updated))
	}
	if len(result.Deleted) != 1 {
		t.Errorf("expected 1 resource deleted, got %d", len(result.Deleted))
	}

	// TODO: Find a way to test methods that use Client Set
	// Test with a wait
	// if err := c.Update("test", objBody(codec, &listB), objBody(codec, &listC), false, 300, true); err != nil {
	// 	t.Fatal(err)
	// }
	// Test with a wait should fail
	// TODO: A way to make this not based off of an extremely short timeout?
	// if err := c.Update("test", objBody(codec, &listC), objBody(codec, &listA), false, 2, true); err != nil {
	// 	t.Fatal(err)
	// }
	expectedActions := []string{
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
	}
	if len(expectedActions) != len(actions) {
		t.Fatalf("unexpected number of requests, expected %d, got %d", len(expectedActions), len(actions))
	}
	for k, v := range expectedActions {
		if actions[k] != v {
			t.Errorf("expected %s request got %s", v, actions[k])
		}
	}
}

func TestUpdate(t *testing.T) {
	testUpdate(t, false)
}

func TestUpdateThreeWayMerge(t *testing.T) {
	testUpdate(t, true)
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
	c.Waiter, err = c.GetWaiter(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(&podList), false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Create(resources)
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
	c.Waiter, err = c.GetWaiter(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(job), false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Create(resources)
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
	c.Waiter, err = c.GetWaiter(LegacyStrategy)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := c.Build(objBody(&pod), false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Create(resources)
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

	kubeClient := k8sfake.NewSimpleClientset(&responsePodList)
	c := Client{Namespace: namespace, kubeClient: kubeClient}

	podList, err := c.GetPodList(namespace, metav1.ListOptions{})
	clientAssertions := assert.New(t)
	clientAssertions.NoError(err)
	clientAssertions.Equal(&responsePodList, podList)
}

func TestOutputContainerLogsForPodList(t *testing.T) {
	namespace := "some-namespace"
	somePodList := newPodList("jimmy", "three", "structs")

	kubeClient := k8sfake.NewSimpleClientset(&somePodList)
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
	// The current state as it exists in the release.
	current *unstructured.Unstructured
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

	patch, patchType, err := createPatch(targetInfo, c.current, c.threeWayMergeForUnstructured)
	if err != nil {
		t.Fatalf("Failed to create patch: %v", err)
	}

	if c.expectedPatch != string(patch) {
		t.Errorf("Unexpected patch.\nTarget:\n%s\nCurrent:\n%s\nActual:\n%s\n\nExpected:\n%s\nGot:\n%s",
			c.target,
			c.current,
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
		name:    "take ownership of resource",
		target:  target,
		current: target,
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
		name:    "merge with spec of existing custom resource",
		target:  target,
		current: target,
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
