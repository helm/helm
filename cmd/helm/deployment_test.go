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

package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kubernetes/helm/pkg/common"
)

type pathAndResponse struct {
	path string
	resp interface{}
}

func TestDeployment(t *testing.T) {
	var deploymentTestCases = []struct {
		args     []string
		resp     []pathAndResponse
		expected string
	}{
		{
			[]string{"deployment", "show", "guestbook.yaml"},
			[]pathAndResponse{{"/deployments/", &common.Deployment{
				Name:  "guestbook.yaml",
				State: &common.DeploymentState{Status: common.CreatedStatus},
			}}},
			"Name: guestbook.yaml\nStatus: Created\n",
		},
		{
			[]string{"deployment", "show", "guestbook.yaml"},
			[]pathAndResponse{{"/deployments/", &common.Deployment{
				Name: "guestbook.yaml",
				State: &common.DeploymentState{
					Status: common.FailedStatus,
					Errors: []string{"error message"},
				},
			}}},
			"Name: guestbook.yaml\nStatus: Failed\nErrors:\n  error message\n",
		},
		{
			[]string{"deployment", "list"},
			[]pathAndResponse{{"/deployments/", []string{"guestbook.yaml"}}},
			"guestbook.yaml\n",
		},
		{
			[]string{"deployment", "describe", "guestbook.yaml"},
			[]pathAndResponse{{
				"/deployments/guestbook.yaml",
				&common.Deployment{Name: "guestbook.yaml",
					State:          &common.DeploymentState{Status: common.CreatedStatus},
					LatestManifest: "manifestxyz",
				}},
				{"/deployments/guestbook.yaml/manifests/manifestxyz", &common.Manifest{
					Deployment: "guestbook.yaml",
					Name:       "manifestxyz",
					ExpandedConfig: &common.Configuration{
						Resources: []*common.Resource{
							{Name: "fe-rc", Type: "ReplicationController", State: &common.ResourceState{Status: common.Created}},
							{Name: "fe", Type: "Service", State: &common.ResourceState{Status: common.Created}},
							{Name: "be-rc", Type: "ReplicationController", State: &common.ResourceState{Status: common.Created}},
							{Name: "be", Type: "Service", State: &common.ResourceState{Status: common.Created}},
						},
					},
				}}},
			"Name:   fe-rc\nType:   ReplicationController\nStatus: Created\n" +
				"Name:   fe\nType:   Service\nStatus: Created\n" +
				"Name:   be-rc\nType:   ReplicationController\nStatus: Created\n" +
				"Name:   be\nType:   Service\nStatus: Created\n",
		},
	}

	for _, tc := range deploymentTestCases {
		th := testHelm(t)
		for _, pathAndResponse := range tc.resp {
			var response = pathAndResponse.resp
			th.mux.HandleFunc(pathAndResponse.path, func(w http.ResponseWriter, r *http.Request) {
				data, err := json.Marshal(response)
				th.must(err)
				w.Write(data)
			})
		}

		th.run(tc.args...)

		if tc.expected != th.output {
			t.Errorf("Expected %v got %v", tc.expected, th.output)
		}
		th.cleanup()
	}
}
