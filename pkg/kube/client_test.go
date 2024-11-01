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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var unstructuredSerializer = resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer
var codec = scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

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

func unprocessableEntityBody() *metav1.Status {
	return &metav1.Status{
		Code:    http.StatusUnprocessableEntity,
		Status:  metav1.StatusFailure,
		Reason:  metav1.StatusReasonInvalid,
		Message: "cannot change",
		Details: &metav1.StatusDetails{},
	}
}

func conflictEntityBody() *metav1.Status {
	return &metav1.Status{
		Code:    http.StatusConflict,
		Status:  metav1.StatusFailure,
		Reason:  metav1.StatusReasonConflict,
		Message: "conflict",
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
	testFactory := cmdtesting.NewTestFactory()
	t.Cleanup(testFactory.Cleanup)

	return &Client{
		Factory: testFactory.WithNamespace("default"),
		Log:     nopLogger,
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
			case path == "/namespaces/default/pods" && method == "POST":
				if strings.Contains(body, "starfish") {
					if iterationCounter < 2 {
						iterationCounter++
						return newResponseJSON(409, resourceQuotaConflict)
					}
					return newResponse(200, &listA.Items[0])
				}
				return newResponseJSON(409, resourceQuotaConflict)
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

func TestUpdate(t *testing.T) {
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
			case p == "/namespaces/default/pods/starfish" && m == "GET":
				return newResponse(200, &listA.Items[0])
			case p == "/namespaces/default/pods/otter" && m == "GET":
				return newResponse(200, &listA.Items[1])
			case p == "/namespaces/default/pods/otter" && m == "PATCH":
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("could not dump request: %s", err)
				}
				req.Body.Close()
				expected := `{}`
				if string(data) != expected {
					t.Errorf("expected patch\n%s\ngot\n%s", expected, string(data))
				}
				return newResponse(200, &listB.Items[0])
			case p == "/namespaces/default/pods/dolphin" && m == "GET":
				return newResponse(404, notFoundBody())
			case p == "/namespaces/default/pods/starfish" && m == "PATCH":
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("could not dump request: %s", err)
				}
				req.Body.Close()
				expected := `{"spec":{"$setElementOrder/containers":[{"name":"app:v4"}],"containers":[{"$setElementOrder/ports":[{"containerPort":443}],"name":"app:v4","ports":[{"containerPort":443,"name":"https"},{"$patch":"delete","containerPort":80}]}]}}`
				if string(data) != expected {
					t.Errorf("expected patch\n%s\ngot\n%s", expected, string(data))
				}
				return newResponse(200, &listB.Items[0])
			case p == "/namespaces/default/pods" && m == "POST":
				if iterationCounter < 2 {
					iterationCounter++
					return newResponseJSON(409, resourceQuotaConflict)
				}
				return newResponse(200, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == "DELETE":
				return newResponse(200, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == "GET":
				return newResponse(200, &listB.Items[2])
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

	result, err := c.UpdateWithTimeout(first, second, false, 0)
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

func TestClient_UpdateWithPolicy(t *testing.T) {

	is := assert.New(t)

	// setup two pods with one diff that should be patched
	original := newPodList("starfish")
	target := newPodList("starfish")
	target.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

	// make sure there is a patchable difference
	is.NotEqual(original.Items[0].Spec.Containers[0].Ports, target.Items[0].Spec.Containers[0].Ports)

	okResponse := func() *http.Response {
		res, _ := newResponse(http.StatusOK, &original.Items[0])
		return res
	}
	notFoundResponse := func() *http.Response {
		res, _ := newResponse(http.StatusNotFound, notFoundBody())
		return res
	}
	unprocessableEntityResponse := func() *http.Response {
		res, _ := newResponse(http.StatusUnprocessableEntity, unprocessableEntityBody())
		return res
	}
	conflictEntityResponse := func() *http.Response {
		res, _ := newResponse(http.StatusConflict, conflictEntityBody())
		return res
	}

	tests := []struct {
		name                 string
		force                bool
		updatePolicy         string
		responses            []*http.Response
		finalResponseFactory func() *http.Response
		expectedActions      []string
		fail                 bool
	}{
		{
			name:         "update should delete and recreate when invalid on PATCH",
			force:        false,
			updatePolicy: updatePolicyRecreateOnInvalid,
			responses: []*http.Response{
				okResponse(),                  // GET
				okResponse(),                  // GET
				unprocessableEntityResponse(), // PATCH
				okResponse(),                  // DELETE
				notFoundResponse(),            // GET
				okResponse(),                  // POST
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PATCH",
				"DELETE",
				"GET",
				"POST",
			},
		},
		{
			name:         "update should delete and recreate when conflict on PATCH",
			force:        false,
			updatePolicy: updatePolicyRecreateOnConflict,
			responses: []*http.Response{
				okResponse(),             // GET
				okResponse(),             // GET
				conflictEntityResponse(), // PATCH
				okResponse(),             // DELETE
				notFoundResponse(),       // GET
				okResponse(),             // POST
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PATCH",
				"DELETE",
				"GET",
				"POST",
			},
		},
		{
			name:         "update should delete and recreate when invalid on PUT",
			force:        true,
			updatePolicy: updatePolicyRecreateOnInvalid,
			responses: []*http.Response{
				okResponse(),                  // GET
				okResponse(),                  // GET
				unprocessableEntityResponse(), // PUT
				okResponse(),                  // DELETE
				notFoundResponse(),            // GET
				okResponse(),                  // POST
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PUT",
				"DELETE",
				"GET",
				"POST",
			},
		},
		{
			name:         "update should delete and recreate when conflict on PUT",
			force:        true,
			updatePolicy: updatePolicyRecreateOnConflict,
			responses: []*http.Response{
				okResponse(),             // GET
				okResponse(),             // GET
				conflictEntityResponse(), // PUT
				okResponse(),             // DELETE
				notFoundResponse(),       // GET
				okResponse(),             // POST
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PUT",
				"DELETE",
				"GET",
				"POST",
			},
		},
		{
			name:         "update should fail when timing out after DELETE",
			force:        false,
			updatePolicy: updatePolicyRecreateOnInvalid,
			responses: []*http.Response{
				okResponse(),                  // GET
				okResponse(),                  // GET
				unprocessableEntityResponse(), // PATCH
				okResponse(),                  // DELETE
			},
			// infinitely return OK on get, implying the server does not delete the resource within the given timeout
			finalResponseFactory: okResponse, // GET
			expectedActions: []string{
				"GET",
				"GET",
				"PATCH",
				"DELETE",
			},
			fail: true,
		},
		{
			name:         "update should fail when no update policy specified",
			force:        false,
			updatePolicy: "",
			responses: []*http.Response{
				okResponse(),                  // GET
				okResponse(),                  // GET
				unprocessableEntityResponse(), // PATCH
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PATCH",
			},
			fail: true,
		},
		{
			name:         "update should fail when no update policy does not match error",
			force:        false,
			updatePolicy: updatePolicyRecreateOnConflict,
			responses: []*http.Response{
				okResponse(),                  // GET
				okResponse(),                  // GET
				unprocessableEntityResponse(), // PATCH
			},
			expectedActions: []string{
				"GET",
				"GET",
				"PATCH",
			},
			fail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actions []string
			c := newTestClient(t)
			c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: unstructuredSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					p, m := req.URL.Path, req.Method
					t.Logf("got request %s %s", p, m)
					if len(tt.responses) == 0 {
						if tt.finalResponseFactory != nil {
							return tt.finalResponseFactory(), nil
						}
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
					// final responses are not added to actions, since it is unpredictable how often the final
					// response will be returned on polling
					actions = append(actions, m)
					var response *http.Response
					response, tt.responses = tt.responses[0], tt.responses[1:]
					return response, nil
				}),
			}

			target.Items[0].Annotations = map[string]string{updatePolicyAnnotation: tt.updatePolicy}

			originalResourceList, err := c.Build(objBody(&original), false)
			if err != nil {
				t.Fatal(err)
			}
			targetResourceList, err := c.Build(objBody(&target), false)
			if err != nil {
				t.Fatal(err)
			}

			// timeout should be larger than one second, to see actual polling with one second poll interval
			_, err = c.UpdateWithTimeout(originalResourceList, targetResourceList, tt.force, 2*time.Second)
			if (tt.fail && err == nil) || (!tt.fail && err != nil) {
				t.Fatal(err)
			}
			t.Log(tt.expectedActions)
			t.Log(actions)
			is.ElementsMatch(tt.expectedActions, actions)
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

	if _, errs := c.Delete(resources); errs != nil {
		t.Fatal(errs)
	}

	resources, err = c.Build(strings.NewReader(testSvcEndpointManifest), false)
	if err != nil {
		t.Fatal(err)
	}
	// ensures that delete does not fail if a resource is not found
	if _, errs := c.Delete(resources); errs != nil {
		t.Fatal(errs)
	}
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
  name: redis-slave
  labels:
    app: redis
    tier: backend
    role: slave
spec:
  ports:
    # the port that this service should serve on
  - port: 6379
  selector:
    app: redis
    tier: backend
    role: slave
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: redis-slave
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: redis
        role: slave
        tier: backend
    spec:
      containers:
      - name: slave
        image: gcr.io/google_samples/gb-redisslave:v1
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
