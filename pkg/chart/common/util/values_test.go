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

package util

import (
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestToRenderValues(t *testing.T) {

	chartValues := map[string]any{
		"name": "al Rashid",
		"where": map[string]any{
			"city":  "Basrah",
			"title": "caliph",
		},
	}

	overrideValues := map[string]any{
		"name": "Haroun",
		"where": map[string]any{
			"city": "Baghdad",
			"date": "809 CE",
		},
	}

	c := &chart.Chart{
		Metadata:  &chart.Metadata{Name: "test"},
		Templates: []*common.File{},
		Values:    chartValues,
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", ModTime: time.Now(), Data: []byte("1,001 Nights")},
		},
	}
	c.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "where"},
	})

	o := common.ReleaseOptions{
		Name:      "Seven Voyages",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}

	res, err := ToRenderValuesWithSchemaValidation(c, overrideValues, o, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the top-level values are all set.
	metamap := res["Chart"].(map[string]any)
	if name := metamap["Name"]; name.(string) != "test" {
		t.Errorf("Expected chart name 'test', got %q", name)
	}
	relmap := res["Release"].(map[string]any)
	if name := relmap["Name"]; name.(string) != "Seven Voyages" {
		t.Errorf("Expected release name 'Seven Voyages', got %q", name)
	}
	if namespace := relmap["Namespace"]; namespace.(string) != "default" {
		t.Errorf("Expected namespace 'default', got %q", namespace)
	}
	if revision := relmap["Revision"]; revision.(int) != 1 {
		t.Errorf("Expected revision '1', got %d", revision)
	}
	if relmap["IsUpgrade"].(bool) {
		t.Error("Expected upgrade to be false.")
	}
	if !relmap["IsInstall"].(bool) {
		t.Errorf("Expected install to be true.")
	}
	if !res["Capabilities"].(*common.Capabilities).APIVersions.Has("v1") {
		t.Error("Expected Capabilities to have v1 as an API")
	}
	if res["Capabilities"].(*common.Capabilities).KubeVersion.Major != "1" {
		t.Error("Expected Capabilities to have a Kube version")
	}

	vals := res["Values"].(common.Values)
	if vals["name"] != "Haroun" {
		t.Errorf("Expected 'Haroun', got %q (%v)", vals["name"], vals)
	}
	where := vals["where"].(map[string]any)
	expects := map[string]string{
		"city":  "Baghdad",
		"date":  "809 CE",
		"title": "caliph",
	}
	for field, expect := range expects {
		if got := where[field]; got != expect {
			t.Errorf("Expected %q, got %q (%v)", expect, got, where)
		}
	}
}
