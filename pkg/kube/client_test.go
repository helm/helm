/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/printers"
	watchjson "k8s.io/kubernetes/pkg/watch/json"
)

func objBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func newPod(name string) core.Pod {
	return newPodWithStatus(name, core.PodStatus{}, "")
}

func newPodWithStatus(name string, status core.PodStatus, namespace string) core.Pod {
	ns := core.NamespaceDefault
	if namespace != "" {
		ns = namespace
	}
	return core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			SelfLink:  "/api/v1/namespaces/default/pods/" + name,
		},
		Spec: core.PodSpec{
			Containers: []core.Container{{
				Name:  "app:v4",
				Image: "abc/app:v4",
				Ports: []core.ContainerPort{{Name: "http", ContainerPort: 80}},
			}},
		},
		Status: status,
	}
}

func newPodList(names ...string) core.PodList {
	var list core.PodList
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
	body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &http.Response{StatusCode: code, Header: header, Body: body}, nil
}

type fakeReaper struct {
	name string
}

func (r *fakeReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	r.name = name
	return nil
}

type fakeReaperFactory struct {
	cmdutil.Factory
	reaper kubectl.Reaper
}

func (f *fakeReaperFactory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	return f.reaper, nil
}

func newEventResponse(code int, e *watch.Event) (*http.Response, error) {
	dispatchedEvent, err := encodeAndMarshalEvent(e)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	body := ioutil.NopCloser(bytes.NewReader(dispatchedEvent))
	return &http.Response{StatusCode: code, Header: header, Body: body}, nil
}

func encodeAndMarshalEvent(e *watch.Event) ([]byte, error) {
	encodedEvent, err := watchjson.Object(testapi.Default.Codec(), e)
	if err != nil {
		return nil, err
	}

	return json.Marshal(encodedEvent)
}

func newTestClient(f cmdutil.Factory) *Client {
	c := New(nil)
	c.Factory = f
	return c
}

func TestUpdate(t *testing.T) {
	listA := newPodList("starfish", "otter", "squid")
	listB := newPodList("starfish", "otter", "dolphin")
	listC := newPodList("starfish", "otter", "dolphin")
	listB.Items[0].Spec.Containers[0].Ports = []core.ContainerPort{{Name: "https", ContainerPort: 443}}
	listC.Items[0].Spec.Containers[0].Ports = []core.ContainerPort{{Name: "https", ContainerPort: 443}}

	var actions []string

	f, tf, codec, _ := cmdtesting.NewAPIFactory()
	tf.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: dynamic.ContentConfig().NegotiatedSerializer,
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
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	reaper := &fakeReaper{}
	rf := &fakeReaperFactory{Factory: f, reaper: reaper}
	c := newTestClient(rf)
	if err := c.Update(core.NamespaceDefault, objBody(codec, &listA), objBody(codec, &listB), false, false, 0, false); err != nil {
		t.Fatal(err)
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
		"/namespaces/default/pods/starfish:PATCH",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/otter:GET",
		"/namespaces/default/pods/dolphin:GET",
		"/namespaces/default/pods:POST",
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

	if reaper.name != "squid" {
		t.Errorf("unexpected reaper: %#v", reaper)
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

	for _, tt := range tests {
		f, _, _, _ := cmdtesting.NewAPIFactory()
		c := newTestClient(f)

		// Test for an invalid manifest
		infos, err := c.Build(tt.namespace, tt.reader)
		if err != nil && !tt.err {
			t.Errorf("%q. Got error message when no error should have occurred: %v", tt.name, err)
		} else if err != nil && strings.Contains(err.Error(), "--validate=false") {
			t.Errorf("%q. error message was not scrubbed", tt.name)
		}

		if len(infos) != tt.count {
			t.Errorf("%q. expected %d result objects, got %d", tt.name, tt.count, len(infos))
		}
	}
}

type testPrinter struct {
	Objects []runtime.Object
	Err     error
	printers.ResourcePrinter
}

func (t *testPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	t.Objects = append(t.Objects, obj)
	fmt.Fprintf(out, "%#v", obj)
	return t.Err
}

func (t *testPrinter) HandledResources() []string {
	return []string{}
}

func (t *testPrinter) AfterPrint(io.Writer, string) error {
	return t.Err
}

func TestGet(t *testing.T) {
	list := newPodList("starfish", "otter")
	f, tf, _, _ := cmdtesting.NewAPIFactory()
	tf.Printer = &testPrinter{}
	tf.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: dynamic.ContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			//actions = append(actions, p+":"+m)
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
	c := newTestClient(f)

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
		results := []*resource.Info{}

		fn := func(info *resource.Info) error {
			results = append(results, info)

			if info.Namespace != tt.namespace {
				t.Errorf("%q. expected namespace to be '%s', got %s", tt.name, tt.namespace, info.Namespace)
			}
			return nil
		}

		f, _, _, _ := cmdtesting.NewAPIFactory()
		c := newTestClient(f)
		infos, err := c.Build(tt.namespace, tt.reader)
		if err != nil && err.Error() != tt.errMessage {
			t.Errorf("%q. Error while building manifests: %v", tt.name, err)
		}

		err = perform(infos, fn)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		if err != nil && err.Error() != tt.errMessage {
			t.Errorf("%q. expected error message: %v, got %v", tt.name, tt.errMessage, err)
		}

		if len(results) != tt.count {
			t.Errorf("%q. expected %d result objects, got %d", tt.name, tt.count, len(results))
		}
	}
}

func TestWaitAndGetCompletedPodPhase(t *testing.T) {
	tests := []struct {
		podPhase      core.PodPhase
		expectedPhase core.PodPhase
		err           bool
		errMessage    string
	}{
		{
			podPhase:      core.PodPending,
			expectedPhase: core.PodUnknown,
			err:           true,
			errMessage:    "watch closed before Until timeout",
		}, {
			podPhase:      core.PodRunning,
			expectedPhase: core.PodUnknown,
			err:           true,
			errMessage:    "watch closed before Until timeout",
		}, {
			podPhase:      core.PodSucceeded,
			expectedPhase: core.PodSucceeded,
		}, {
			podPhase:      core.PodFailed,
			expectedPhase: core.PodFailed,
		},
	}

	for _, tt := range tests {
		f, tf, codec, ns := cmdtesting.NewAPIFactory()
		actions := make(map[string]string)

		var testPodList core.PodList
		testPodList.Items = append(testPodList.Items, newPodWithStatus("bestpod", core.PodStatus{Phase: tt.podPhase}, "test"))

		tf.Client = &fake.RESTClient{
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				p, m := req.URL.Path, req.Method
				actions[p] = m
				switch {
				case p == "/namespaces/test/pods/bestpod" && m == "GET":
					return newResponse(200, &testPodList.Items[0])
				case p == "/namespaces/test/pods" && m == "GET":
					event := watch.Event{Type: watch.Added, Object: &testPodList.Items[0]}
					return newEventResponse(200, &event)
				default:
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
					return nil, nil
				}
			}),
		}

		c := newTestClient(f)

		phase, err := c.WaitAndGetCompletedPodPhase("test", objBody(codec, &testPodList), 1*time.Second)
		if (err != nil) != tt.err {
			t.Fatalf("Expected error but there was none.")
		}
		if err != nil && err.Error() != tt.errMessage {
			t.Fatalf("Expected error %s, got %s", tt.errMessage, err.Error())
		}
		if phase != tt.expectedPhase {
			t.Fatalf("Expected pod phase %s, got %s", tt.expectedPhase, phase)
		}
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
