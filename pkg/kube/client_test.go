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
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	api "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestUpdateResource(t *testing.T) {

	tests := []struct {
		name       string
		namespace  string
		modified   *resource.Info
		currentObj runtime.Object
		err        bool
		errMessage string
	}{
		{
			name:       "no changes when updating resources",
			modified:   createFakeInfo("nginx", nil),
			currentObj: createFakePod("nginx", nil),
			err:        true,
			errMessage: "Looks like there are no changes for nginx",
		},
		//{
		//name:       "valid update input",
		//modified:   createFakeInfo("nginx", map[string]string{"app": "nginx"}),
		//currentObj: createFakePod("nginx", nil),
		//},
	}

	for _, tt := range tests {
		err := updateResource(tt.modified, tt.currentObj)
		if err != nil && err.Error() != tt.errMessage {
			t.Errorf("%q. expected error message: %v, got %v", tt.name, tt.errMessage, err)
		}
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
			errMessage: "no objects passed to create",
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

		c := New(nil)
		c.ClientForMapping = func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
			return &fake.RESTClient{}, nil
		}

		err := perform(c, tt.namespace, tt.reader, fn)
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

func TestReal(t *testing.T) {
	t.Skip("This is a live test, comment this line to run")
	if err := New(nil).Create("test", strings.NewReader(guestbookManifest)); err != nil {
		t.Fatal(err)
	}

	testSvcEndpointManifest := testServiceManifest + "\n---\n" + testEndpointManifest
	c := New(nil)
	if err := c.Create("test-delete", strings.NewReader(testSvcEndpointManifest)); err != nil {
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
        image: gcr.io/google_containers/redis:e2e  # or just image: redis
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

func createFakePod(name string, labels map[string]string) runtime.Object {
	objectMeta := createObjectMeta(name, labels)

	object := &api.Pod{
		ObjectMeta: objectMeta,
	}

	return object
}

func createFakeInfo(name string, labels map[string]string) *resource.Info {
	pod := createFakePod(name, labels)
	marshaledObj, _ := json.Marshal(pod)

	mapping := &meta.RESTMapping{
		Resource: name,
		Scope:    meta.RESTScopeNamespace,
		GroupVersionKind: unversioned.GroupVersionKind{
			Kind:    "Pod",
			Version: "v1",
		}}

	client := &fake.RESTClient{
		Codec: testapi.Default.Codec(),
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			header := http.Header{}
			header.Set("Content-Type", runtime.ContentTypeJSON)
			return &http.Response{
				StatusCode: 200,
				Header:     header,
				Body:       ioutil.NopCloser(bytes.NewReader(marshaledObj)),
			}, nil
		})}
	info := resource.NewInfo(client, mapping, "default", "nginx", false)

	info.Object = pod

	return info
}

func createObjectMeta(name string, labels map[string]string) api.ObjectMeta {
	objectMeta := api.ObjectMeta{Name: name, Namespace: "default"}

	if labels != nil {
		objectMeta.Labels = labels
	}

	return objectMeta
}
