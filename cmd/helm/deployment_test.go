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

func TestShowDeployment(t *testing.T) {
	th := setup()
	defer th.teardown()

	deployment := common.NewDeployment("guestbook.yaml")

	th.mux.HandleFunc("/deployments/", func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(deployment)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	})

	expected := "Name: guestbook.yaml\nStatus: Created\n"

	actual := CaptureOutput(func() {
		th.Run("deployment", "show", "guestbook.yaml")
	})

	if expected != actual {
		t.Errorf("Expected %v got %v", expected, actual)
	}
}

func TestListDeployment(t *testing.T) {
	th := setup()
	defer th.teardown()

	th.mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["guestbook.yaml"]`))
	})

	expected := "guestbook.yaml\n"

	actual := CaptureOutput(func() {
		th.Run("deployment", "list")
	})

	if expected != actual {
		t.Errorf("Expected %v got %v", expected, actual)
	}
}
