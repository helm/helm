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

func TestPostDeployment(t *testing.T) {
	cfg := &common.Configuration{
		Resources: []*common.Resource{
			{
				Name: "foo",
				Type: "helm:example.com/foo/bar",
				Properties: map[string]interface{}{
					"port": ":8080",
				},
			},
		},
	}

	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, "{}")
		}),
	}
	defer fc.teardown()

	if err := fc.setup().PostDeployment("foo", cfg); err != nil {
		t.Fatalf("failed to post deployment: %s", err)
	}
}
