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

package manager

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes/deployment-manager/util"

	"github.com/ghodss/yaml"
)

var validConfigurationTestCaseData = []byte(`
resources:
  - name: test-controller-v1
    type: ReplicationController
    properties:
      kind: ReplicationController
      apiVersion: v1
      metadata: 
        name: test-controller-v1
        namespace: dm
        labels: 
          k8s-app: test
          version: v1
      spec: 
        replicas: 1
        selector: 
          k8s-app: test
          version: v1
        template: 
          metadata: 
            labels: 
              k8s-app: test
              version: v1
          spec: 
            containers: 
              - name: test
                image: deployer/test:latest
                ports: 
                  - name: test
                    containerPort: 8080
                    protocol: TCP
  - name: test
    type: Service
    properties:
      apiVersion: v1
      kind: Service
      metadata: 
        name: test
        namespace: dm
        labels: 
          k8s-app: test
          version: v1
      spec: 
        type: LoadBalancer
        selector: 
          k8s-app: test
          version: v1
        ports: 
          - name: test
            port: 8080
            targetPort: test
            protocol: TCP
`)

type DeployerTestCases struct {
	TestCases []DeployerTestCase
}

type DeployerTestCase struct {
	Description string
	Error       string
	Handler     func(w http.ResponseWriter, r *http.Request)
}

func TestGetConfiguration(t *testing.T) {
	valid := getValidConfiguration(t)
	tests := []DeployerTestCase{
		{
			"expect success for GetConfiguration",
			"",
			func(w http.ResponseWriter, r *http.Request) {
				// Get name from path, find in valid, and return its properties.
				rtype := path.Base(path.Dir(r.URL.Path))
				rname := path.Base(r.URL.Path)
				for _, resource := range valid.Resources {
					if resource.Type == rtype && resource.Name == rname {
						util.LogHandlerExitWithYAML("resourcifier: get configuration", w, resource.Properties, http.StatusOK)
						return
					}
				}

				status := fmt.Sprintf("resource %s of type %s not found", rname, rtype)
				http.Error(w, status, http.StatusInternalServerError)
			},
		},
		{
			"expect error for GetConfiguration",
			"cannot get configuration",
			deployerErrorHandler,
		},
	}

	for _, dtc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(dtc.Handler))
		defer ts.Close()

		deployer := NewDeployer(ts.URL)
		result, err := deployer.GetConfiguration(valid)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, dtc.Error) {
				t.Errorf("error in test case:%s:%s\n", dtc.Description, message)
			}
		} else {
			if dtc.Error != "" {
				t.Errorf("expected error:%s\ndid not occur in test case:%s\n",
					dtc.Error, dtc.Description)
			}

			if !reflect.DeepEqual(valid, result) {
				t.Errorf("error in test case:%s:\nwant:%s\nhave:%s\n",
					dtc.Description, util.ToYAMLOrError(valid), util.ToYAMLOrError(result))
			}
		}
	}
}

func TestCreateConfiguration(t *testing.T) {
	valid := getValidConfiguration(t)
	tests := []DeployerTestCase{
		{
			"expect success for CreateConfiguration",
			"",
			deployerSuccessHandler,
		},
		{
			"expect error for CreateConfiguration",
			"cannot create configuration",
			deployerErrorHandler,
		},
	}

	for _, dtc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(dtc.Handler))
		defer ts.Close()

		deployer := NewDeployer(ts.URL)
		err := deployer.CreateConfiguration(valid)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, dtc.Error) {
				t.Errorf("error in test case:%s:%s\n", dtc.Description, message)
			}
		} else {
			if dtc.Error != "" {
				t.Errorf("expected error:%s\ndid not occur in test case:%s\n",
					dtc.Error, dtc.Description)
			}
		}
	}
}

func TestDeleteConfiguration(t *testing.T) {
	valid := getValidConfiguration(t)
	tests := []DeployerTestCase{
		{
			"expect success for DeleteConfiguration",
			"",
			deployerSuccessHandler,
		},
		{
			"expect error for DeleteConfiguration",
			"cannot delete configuration",
			deployerErrorHandler,
		},
	}

	for _, dtc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(dtc.Handler))
		defer ts.Close()

		deployer := NewDeployer(ts.URL)
		err := deployer.DeleteConfiguration(valid)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, dtc.Error) {
				t.Errorf("error in test case:%s:%s\n", dtc.Description, message)
			}
		} else {
			if dtc.Error != "" {
				t.Errorf("expected error:%s\ndid not occur in test case:%s\n",
					dtc.Error, dtc.Description)
			}
		}
	}
}

func TestPutConfiguration(t *testing.T) {
	valid := getValidConfiguration(t)
	tests := []DeployerTestCase{
		{
			"expect success for PutConfiguration",
			"",
			deployerSuccessHandler,
		},
		{
			"expect error for PutConfiguration",
			"cannot replace configuration",
			deployerErrorHandler,
		},
	}

	for _, dtc := range tests {
		ts := httptest.NewServer(http.HandlerFunc(dtc.Handler))
		defer ts.Close()

		deployer := NewDeployer(ts.URL)
		err := deployer.PutConfiguration(valid)
		if err != nil {
			message := err.Error()
			if !strings.Contains(message, dtc.Error) {
				t.Errorf("error in test case:%s:%s\n", dtc.Description, message)
			}
		} else {
			if dtc.Error != "" {
				t.Errorf("expected error:%s\ndid not occur in test case:%s\n",
					dtc.Error, dtc.Description)
			}
		}
	}
}

func getValidConfiguration(t *testing.T) *Configuration {
	valid := &Configuration{}
	err := yaml.Unmarshal(validConfigurationTestCaseData, valid)
	if err != nil {
		t.Errorf("cannot unmarshal test case data:%s\n", err)
	}

	return valid
}

func deployerErrorHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	http.Error(w, "something failed", http.StatusInternalServerError)
}

func deployerSuccessHandler(w http.ResponseWriter, r *http.Request) {
	valid := &Configuration{}
	err := yaml.Unmarshal(validConfigurationTestCaseData, valid)
	if err != nil {
		status := fmt.Sprintf("cannot unmarshal test case data:%s", err)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		status := fmt.Sprintf("cannot read request body:%s", err)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	result := &Configuration{}
	if err := yaml.Unmarshal(body, result); err != nil {
		status := fmt.Sprintf("cannot unmarshal request body:%s", err)
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	if !reflect.DeepEqual(valid, result) {
		status := fmt.Sprintf("error in http handler:\nwant:%s\nhave:%s\n",
			util.ToYAMLOrError(valid), util.ToYAMLOrError(result))
		http.Error(w, status, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
