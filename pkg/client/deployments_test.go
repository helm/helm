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

package client

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/kubernetes/helm/pkg/common"
)

func TestListDeployments(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`["guestbook.yaml"]`))
		}),
	}
	defer fc.teardown()

	l, err := fc.setup().ListDeployments()
	if err != nil {
		t.Fatal(err)
	}

	if len(l) != 1 {
		t.Fatal("expected a single deployment")
	}
}

func TestGetDeployment(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"name":"guestbook.yaml","id":0,"createdAt":"2016-02-08T12:17:49.251658308-08:00","deployedAt":"2016-02-08T12:17:49.251658589-08:00","modifiedAt":"2016-02-08T12:17:51.177518098-08:00","deletedAt":"0001-01-01T00:00:00Z","state":{"status":"Deployed"},"latestManifest":"manifest-1454962670728402229"}`))
		}),
	}
	defer fc.teardown()

	d, err := fc.setup().GetDeployment("guestbook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if d.Name != "guestbook.yaml" {
		t.Fatalf("expected deployment name 'guestbook.yaml', got '%s'", d.Name)
	}

	if d.State.Status != common.DeployedStatus {
		t.Fatalf("expected deployment status 'Deployed', got '%s'", d.State.Status)
	}
}

func TestDescribeDeployment(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.String(), "manifest") {
				w.Write([]byte(`{"deployment":"guestbook.yaml","name":"manifest-1454962670728402229","expandedConfig":{"resources":[{"name":"fe-rc","type":"ReplicationController","state":{"status":"Created"}},{"name":"fe","type":"Service","state":{"status":"Created"}}]}}`))
			} else {
				w.Write([]byte(`{"name":"guestbook.yaml","id":0,"createdAt":"2016-02-08T12:17:49.251658308-08:00","deployedAt":"2016-02-08T12:17:49.251658589-08:00","modifiedAt":"2016-02-08T12:17:51.177518098-08:00","deletedAt":"0001-01-01T00:00:00Z","state":{"status":"Deployed"},"latestManifest":"manifest-1454962670728402229"}`))
			}

		}),
	}
	defer fc.teardown()

	m, err := fc.setup().DescribeDeployment("guestbook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if m.Deployment != "guestbook.yaml" {
		t.Fatalf("expected deployment name 'guestbook.yaml', got '%s'", m.Name)
	}
	if m.Name != "manifest-1454962670728402229" {
		t.Fatalf("expected manifest name 'manifest-1454962670728402229', got '%s'", m.Name)
	}
	if len(m.ExpandedConfig.Resources) != 2 {
		t.Fatalf("expected two resources, got %d", len(m.ExpandedConfig.Resources))
	}
	var foundFE = false
	var foundFERC = false
	for _, r := range m.ExpandedConfig.Resources {
		if r.Name == "fe" {
			foundFE = true
			if r.Type != "Service" {
				t.Fatalf("Incorrect type, expected 'Service' got '%s'", r.Type)
			}
		}
		if r.Name == "fe-rc" {
			foundFERC = true
			if r.Type != "ReplicationController" {
				t.Fatalf("Incorrect type, expected 'ReplicationController' got '%s'", r.Type)
			}
		}
		if r.State.Status != common.Created {
			t.Fatalf("Incorrect status, expected '%s' got '%s'", common.Created, r.State.Status)
		}
	}
	if !foundFE {
		t.Fatalf("didn't find 'fe' in resources")
	}
	if !foundFERC {
		t.Fatalf("didn't find 'fe-rc' in resources")
	}
}

func TestDescribeDeploymentFailedDeployment(t *testing.T) {
	var expectedError = "Deployment: 'guestbook.yaml' has no manifest"
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"name":"guestbook.yaml","createdAt":"2016-04-02T10:41:06.509049871-07:00","deployedAt":"0001-01-01T00:00:00Z","modifiedAt":"2016-04-02T10:41:06.509203582-07:00","deletedAt":"0001-01-01T00:00:00Z","state":{"status":"Failed","errors":["cannot expand configuration:No repository for url gs://kubernetes-charts-testing/redis-2.tgz\n\u0026{[0xc82014efc0]}\n"]},"latestManifest":""}`))
		}),
	}
	defer fc.teardown()

	m, err := fc.setup().DescribeDeployment("guestbook.yaml")
	if err == nil {
		t.Fatal("Did not get an error for missing manifest")
	}
	if err.Error() != expectedError {
		t.Fatalf("Unexpected error message, wanted:\n%s\ngot:\n%s", expectedError, err.Error())
	}
	if m != nil {
		t.Fatal("Got back manifest but shouldn't have")
	}
}

func TestPostDeployment(t *testing.T) {
	chartInvocation := &common.Resource{
		Name: "foo",
		Type: "helm:example.com/foo/bar",
		Properties: map[string]interface{}{
			"port": ":8080",
		},
	}

	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, "{}")
		}),
	}
	defer fc.teardown()

	if err := fc.setup().PostDeployment(chartInvocation); err != nil {
		t.Fatalf("failed to post deployment: %s", err)
	}
}
