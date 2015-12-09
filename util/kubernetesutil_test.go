/*
Copyright 2015 The Kubernetes Authors All rights reserved.
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

package util

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ghodss/yaml"

	"github.com/kubernetes/deployment-manager/common"
)

var serviceInput = `
  kind: "Service"
  apiVersion: "v1"
  metadata:
    name: "mock"
    labels:
      app: "mock"
  spec:
    ports:
      -
        protocol: "TCP"
        port: 99
        targetPort: 9949
    selector:
      app: "mock"
`

var serviceExpected = `
name: mock
type: Service
properties:
    kind: "Service"
    apiVersion: "v1"
    metadata:
      name: "mock"
      labels:
        app: "mock"
    spec:
      ports:
        -
          protocol: "TCP"
          port: 99
          targetPort: 9949
      selector:
        app: "mock"
`

var rcInput = `
  kind: "ReplicationController"
  apiVersion: "v1"
  metadata:
    name: "mockname"
    labels:
      app: "mockapp"
      foo: "bar"
  spec:
    replicas: 1
    selector:
      app: "mockapp"
    template:
      metadata:
        labels:
          app: "mocklabel"
      spec:
        containers:
          -
            name: "mock-container"
            image: "kubernetes/pause"
            ports:
              -
                containerPort: 9949
                protocol: "TCP"
              -
                containerPort: 9949
                protocol: "TCP"
`

var rcExpected = `
name: mockname
type: ReplicationController
properties:
    kind: "ReplicationController"
    apiVersion: "v1"
    metadata:
      name: "mockname"
      labels:
        app: "mockapp"
        foo: "bar"
    spec:
      replicas: 1
      selector:
        app: "mockapp"
      template:
        metadata:
          labels:
            app: "mocklabel"
        spec:
          containers:
            -
              name: "mock-container"
              image: "kubernetes/pause"
              ports:
                -
                  containerPort: 9949
                  protocol: "TCP"
                -
                  containerPort: 9949
                  protocol: "TCP"
`

func unmarshalResource(t *testing.T, object []byte) (*common.Resource, error) {
	r := &common.Resource{}
	if err := yaml.Unmarshal([]byte(object), &r); err != nil {
		t.Errorf("cannot unmarshal test object (%#v)", err)
		return nil, err
	}
	return r, nil
}

func testConversion(t *testing.T, object []byte, expected []byte) {
	e, err := unmarshalResource(t, expected)
	if err != nil {
		t.Fatalf("Failed to unmarshal expected Resource: %v", err)
	}

	result, err := ParseKubernetesObject(object)
	if err != nil {
		t.Fatalf("ParseKubernetesObject failed: %v")
	}
	// Since the object name gets created on the fly, we have to rejigger the returned object
	// slightly to make sure the DeepEqual works as expected.
	// First validate the name matches the expected format.
	var i int
	format := e.Name + "-%d"
	count, err := fmt.Sscanf(result.Name, format, &i)
	if err != nil || count != 1 {
		t.Errorf("Name is not as expected, wanted of the form %s got %s", format, result.Name)
	}
	e.Name = result.Name
	if !reflect.DeepEqual(result, e) {
		t.Errorf("expected %+v but found %+v", e, result)
	}

}

func TestSimple(t *testing.T) {
	testConversion(t, []byte(rcInput), []byte(rcExpected))
	testConversion(t, []byte(serviceInput), []byte(serviceExpected))
}
