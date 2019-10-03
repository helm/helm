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
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	kubectlscheme "k8s.io/kubernetes/pkg/kubectl/scheme"
)

func init() {
	err := apiextv1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// Tiller use the scheme from go-client, but the cmdtesting
	// package used here is hardcoded to use the scheme from
	// kubectl. So for testing, we need to add the CustomResourceDefinition
	// type to both schemes.
	err = apiextv1beta1.AddToScheme(kubectlscheme.Scheme)
	if err != nil {
		panic(err)
	}
}

var (
	unstructuredSerializer = resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer
)

func getCodec() runtime.Codec {
	return scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
}

func objBody(obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(getCodec(), obj))))
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

func newService(name string) v1.Service {
	ns := v1.NamespaceDefault
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			SelfLink:  "/api/v1/namespaces/default/services/" + name,
		},
		Spec: v1.ServiceSpec{},
	}
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
	body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(getCodec(), obj))))
	return &http.Response{StatusCode: code, Header: header, Body: body}, nil
}

type testClient struct {
	*Client
	*cmdtesting.TestFactory
}

func newTestClient() *testClient {
	tf := cmdtesting.NewTestFactory()
	c := &Client{
		Factory: tf,
		Log:     nopLogger,
	}
	return &testClient{
		Client:      c,
		TestFactory: tf,
	}
}

func TestUpdate(t *testing.T) {
	listA := newPodList("starfish", "otter", "squid")
	listB := newPodList("starfish", "otter", "dolphin")
	listC := newPodList("starfish", "otter", "dolphin")
	listB.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}
	listC.Items[0].Spec.Containers[0].Ports = []v1.ContainerPort{{Name: "https", ContainerPort: 443}}

	var actions []string

	tf := cmdtesting.NewTestFactory()
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
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
			case p == "/namespaces/default/pods/dolphin" && m == "GET":
				return newResponse(404, notFoundBody())
			case p == "/namespaces/default/pods/starfish" && m == "PATCH":
				data, err := ioutil.ReadAll(req.Body)
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
				return newResponse(200, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == "DELETE":
				return newResponse(200, &listB.Items[1])
			case p == "/namespaces/default/pods/squid" && m == "GET":
				return newResponse(200, &listA.Items[2])
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	c := &Client{
		Factory: tf,
		Log:     nopLogger,
	}

	if err := c.Update(v1.NamespaceDefault, objBody(&listA), objBody(&listB), false, false, 0, false); err != nil {
		t.Fatal(err)
	}
	// TODO: Find a way to test methods that use Client Set
	// Test with a wait
	// if err := c.Update("test", objBody(&listB), objBody(&listC), false, 300, true); err != nil {
	// 	t.Fatal(err)
	// }
	// Test with a wait should fail
	// TODO: A way to make this not based off of an extremely short timeout?
	// if err := c.Update("test", objBody(&listC), objBody(&listA), false, 2, true); err != nil {
	// 	t.Fatal(err)
	// }
	expectedActions := []string{
		"/namespaces/default/pods/starfish:GET",
		"/namespaces/default/pods/starfish:PATCH",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/dolphin:GET",
		"/namespaces/default/pods:POST",
		"/namespaces/default/pods/squid:GET",
		"/namespaces/default/pods/squid:DELETE",
	}
	if len(expectedActions) != len(actions) {
		t.Errorf("unexpected number of requests, expected %d, got %d", len(expectedActions), len(actions))
		return
	}
	for k, v := range expectedActions {
		if actions[k] != v {
			t.Errorf("expected %s request got %s", v, actions[k])
		}
	}

	// Test resource policy is respected
	actions = nil
	listA.Items[2].ObjectMeta.Annotations = map[string]string{ResourcePolicyAnno: "keep"}
	if err := c.Update(v1.NamespaceDefault, objBody(&listA), objBody(&listB), false, false, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, v := range actions {
		if v == "/namespaces/default/pods/squid:DELETE" {
			t.Errorf("should not have deleted squid - it has helm.sh/resource-policy=keep")
		}
	}
}

func TestUpdateNonManagedResourceError(t *testing.T) {
	actual := newPodList("starfish")
	current := newPodList()
	target := newPodList("starfish")

	tf := cmdtesting.NewTestFactory()
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/starfish" && m == "GET":
				return newResponse(200, &actual.Items[0])
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	c := &Client{
		Factory: tf,
		Log:     nopLogger,
	}

	if err := c.Update(v1.NamespaceDefault, objBody(&current), objBody(&target), false, false, 0, false); err != nil {
		if err.Error() != "kind Pod with the name \"starfish\" already exists in the cluster and wasn't defined in the previous release. Before upgrading, please either delete the resource from the cluster or remove it from the chart" {
			t.Fatal(err)
		}
	} else {
		t.Fatalf("error expected")
	}
}

func TestDeleteWithTimeout(t *testing.T) {
	testCases := map[string]struct {
		deleteTimeout int64
		deleteAfter   time.Duration
		success       bool
	}{
		"resource is deleted within timeout period": {
			int64((2 * time.Minute).Seconds()),
			10 * time.Second,
			true,
		},
		"resource is not deleted within the timeout period": {
			int64((10 * time.Second).Seconds()),
			20 * time.Second,
			false,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			c := newTestClient()
			defer c.Cleanup()

			service := newService("my-service")
			startTime := time.Now()
			c.TestFactory.UnstructuredClient = &fake.RESTClient{
				GroupVersion:         schema.GroupVersion{Version: "v1"},
				NegotiatedSerializer: unstructuredSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					currentTime := time.Now()
					if startTime.Add(tc.deleteAfter).Before(currentTime) {
						return newResponse(404, notFoundBody())
					}
					return newResponse(200, &service)
				}),
			}

			err := c.DeleteWithTimeout(metav1.NamespaceDefault, strings.NewReader(testServiceManifest), tc.deleteTimeout, true)
			if err != nil && tc.success {
				t.Errorf("expected no error, but got %v", err)
			}
			if err == nil && !tc.success {
				t.Errorf("expected error, but didn't get one")
			}
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
			name:      "Invalid schema",
			namespace: "test",
			reader:    strings.NewReader(testInvalidServiceManifest),
			err:       true,
		},
	}

	c := newTestClient()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Cleanup()

			// Test for an invalid manifest
			infos, err := c.Build(tt.namespace, tt.reader)
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

func TestGet(t *testing.T) {
	list := newPodList("starfish", "otter")
	c := newTestClient()
	defer c.Cleanup()
	c.TestFactory.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/starfish" && m == "GET":
				return newResponse(404, notFoundBody())
			case p == "/namespaces/default/pods/otter" && m == "GET":
				return newResponse(200, &list.Items[1])
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	// Test Success
	data := strings.NewReader("kind: Pod\napiVersion: v1\nmetadata:\n  name: otter")
	o, err := c.Get("default", data)
	if err != nil {
		t.Errorf("Expected missing results, got %q", err)
	}
	if !strings.Contains(o, "==> v1/Pod") && !strings.Contains(o, "otter") {
		t.Errorf("Expected v1/Pod otter, got %s", o)
	}

	// Test failure
	data = strings.NewReader("kind: Pod\napiVersion: v1\nmetadata:\n  name: starfish")
	o, err = c.Get("default", data)
	if err != nil {
		t.Errorf("Expected missing results, got %q", err)
	}
	if !strings.Contains(o, "MISSING") && !strings.Contains(o, "pods\t\tstarfish") {
		t.Errorf("Expected missing starfish, got %s", o)
	}
}

func TestResourceTypeSortOrder(t *testing.T) {
	pod := newPod("my-pod")
	service := newService("my-service")
	c := newTestClient()
	defer c.Cleanup()
	c.TestFactory.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/my-pod" && m == "GET":
				return newResponse(200, &pod)
			case p == "/namespaces/default/services/my-service" && m == "GET":
				return newResponse(200, &service)
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	// Test sorting order
	data := strings.NewReader(testResourceTypeSortOrder)
	o, err := c.Get("default", data)
	if err != nil {
		t.Errorf("Expected missing results, got %q", err)
	}
	podIndex := strings.Index(o, "my-pod")
	serviceIndex := strings.Index(o, "my-service")
	if podIndex == -1 {
		t.Errorf("Expected v1/Pod my-pod, got %s", o)
	}
	if serviceIndex == -1 {
		t.Errorf("Expected v1/Service my-service, got %s", o)
	}
	if !sort.IntsAreSorted([]int{podIndex, serviceIndex}) {
		t.Errorf("Expected order: [v1/Pod v1/Service], got %s", o)
	}
}

func TestResourceSortOrder(t *testing.T) {
	list := newPodList("albacore", "coral", "beluga")
	c := newTestClient()
	defer c.Cleanup()
	c.TestFactory.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: unstructuredSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			switch {
			case p == "/namespaces/default/pods/albacore" && m == "GET":
				return newResponse(200, &list.Items[0])
			case p == "/namespaces/default/pods/coral" && m == "GET":
				return newResponse(200, &list.Items[1])
			case p == "/namespaces/default/pods/beluga" && m == "GET":
				return newResponse(200, &list.Items[2])
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	// Test sorting order
	data := strings.NewReader(testResourceSortOrder)
	o, err := c.Get("default", data)
	if err != nil {
		t.Errorf("Expected missing results, got %q", err)
	}
	albacoreIndex := strings.Index(o, "albacore")
	belugaIndex := strings.Index(o, "beluga")
	coralIndex := strings.Index(o, "coral")
	if albacoreIndex == -1 {
		t.Errorf("Expected v1/Pod albacore, got %s", o)
	}
	if belugaIndex == -1 {
		t.Errorf("Expected v1/Pod beluga, got %s", o)
	}
	if coralIndex == -1 {
		t.Errorf("Expected v1/Pod coral, got %s", o)
	}
	if !sort.IntsAreSorted([]int{albacoreIndex, belugaIndex, coralIndex}) {
		t.Errorf("Expected order: [albacore beluga coral], got %s", o)
	}
}

func TestWaitUntilCRDEstablished(t *testing.T) {
	testCases := map[string]struct {
		conditions            []apiextv1beta1.CustomResourceDefinitionCondition
		returnConditionsAfter int
		success               bool
	}{
		"crd reaches established state after 2 requests": {
			conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
				{
					Type:   apiextv1beta1.Established,
					Status: apiextv1beta1.ConditionTrue,
				},
			},
			returnConditionsAfter: 2,
			success:               true,
		},
		"crd does not reach established state before timeout": {
			conditions:            []apiextv1beta1.CustomResourceDefinitionCondition{},
			returnConditionsAfter: 100,
			success:               false,
		},
		"crd name is not accepted": {
			conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
				{
					Type:   apiextv1beta1.NamesAccepted,
					Status: apiextv1beta1.ConditionFalse,
				},
			},
			returnConditionsAfter: 1,
			success:               false,
		},
	}

	for tn, tc := range testCases {
		func(name string) {
			c := newTestClient()
			defer c.Cleanup()

			crdWithoutConditions := newCrdWithStatus("name", apiextv1beta1.CustomResourceDefinitionStatus{})
			crdWithConditions := newCrdWithStatus("name", apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: tc.conditions,
			})

			requestCount := 0
			c.TestFactory.UnstructuredClient = &fake.RESTClient{
				GroupVersion:         schema.GroupVersion{Version: "v1"},
				NegotiatedSerializer: unstructuredSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					var crd apiextv1beta1.CustomResourceDefinition
					if requestCount < tc.returnConditionsAfter {
						crd = crdWithoutConditions
					} else {
						crd = crdWithConditions
					}
					requestCount++
					return newResponse(200, &crd)
				}),
			}

			err := c.WaitUntilCRDEstablished(strings.NewReader(crdManifest), 5*time.Second)
			if err != nil && tc.success {
				t.Errorf("%s: expected no error, but got %v", name, err)
			}
			if err == nil && !tc.success {
				t.Errorf("%s: expected error, but didn't get one", name)
			}
		}(tn)
	}
}

func newCrdWithStatus(name string, status apiextv1beta1.CustomResourceDefinitionStatus) apiextv1beta1.CustomResourceDefinition {
	crd := apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec:   apiextv1beta1.CustomResourceDefinitionSpec{},
		Status: status,
	}
	return crd
}

func TestPerform(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		reader     io.Reader
		count      int
		err        bool
		errMessage string
	}{
		{
			name:      "Valid input",
			namespace: "test",
			reader:    strings.NewReader(guestbookManifest),
			count:     6,
		}, {
			name:       "Empty manifests",
			namespace:  "test",
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

				if info.Namespace != tt.namespace {
					t.Errorf("expected namespace to be '%s', got %s", tt.namespace, info.Namespace)
				}
				return nil
			}

			c := newTestClient()
			defer c.Cleanup()
			infos, err := c.Build(tt.namespace, tt.reader)
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
	if err := c.Create("test", strings.NewReader(guestbookManifest), 300, false); err != nil {
		t.Fatal(err)
	}

	testSvcEndpointManifest := testServiceManifest + "\n---\n" + testEndpointManifest
	c = New(nil)
	if err := c.Create("test-delete", strings.NewReader(testSvcEndpointManifest), 300, false); err != nil {
		t.Fatal(err)
	}

	if err := c.Delete("test-delete", strings.NewReader(testEndpointManifest)); err != nil {
		t.Fatal(err)
	}

	// ensures that delete does not fail if a resource is not found
	if err := c.Delete("test-delete", strings.NewReader(testSvcEndpointManifest)); err != nil {
		t.Fatal(err)
	}
}

const testResourceTypeSortOrder = `
kind: Service
apiVersion: v1
metadata:
  name: my-service
---
kind: Pod
apiVersion: v1
metadata:
  name: my-pod
`

const testResourceSortOrder = `
kind: Pod
apiVersion: v1
metadata:
  name: albacore
---
kind: Pod
apiVersion: v1
metadata:
  name: coral
---
kind: Pod
apiVersion: v1
metadata:
  name: beluga
`

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

const testInvalidServiceManifest = `
kind: Service
apiVersion: v1
spec:
  ports:
    - port: "80"
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
apiVersion: apps/v1
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
        image: k8s.gcr.io/redis:e2e  # or just image: redis
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
apiVersion: apps/v1
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
apiVersion: apps/v1
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

const crdManifest = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  labels:
    controller-tools.k8s.io: "1.0"
  name: applications.app.k8s.io
spec:
  group: app.k8s.io
  names:
    kind: Application
    plural: applications
  scope: Namespaced
  validation:
    openAPIV3Schema:
      properties:
        apiVersion:
          description: 'Description'
          type: string
        kind:
          description: 'Kind'
          type: string
        metadata:
          type: object
        spec:
          type: object
        status:
          type: object
  version: v1beta1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`
