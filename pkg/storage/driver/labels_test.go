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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"testing"
)

func TestLabelsMatch(t *testing.T) {
	var tests = []struct {
		desc   string
		set1   labels
		set2   labels
		expect bool
	}{
		{
			"equal labels sets",
			labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
			labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
			true,
		},
		{
			"disjoint label sets",
			labels(map[string]string{"KEY_C": "VAL_C", "KEY_D": "VAL_D"}),
			labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
			false,
		},
	}

	for _, tt := range tests {
		if !tt.set1.match(tt.set2) && tt.expect {
			t.Fatalf("Expected match '%s'\n", tt.desc)
		}
	}
}

func TestValidate(t *testing.T) {
	// empty map
	var nilMap map[string]string
	if err := validate(nilMap); err != nil {
		t.Errorf("Nil label map should not fail when validating: %s", err)
	}
	// customized labels have no preserved labels of Helm
	labels0 := map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}
	if err := validate(labels0); err != nil {
		t.Errorf("Customized labels with no preserved labels of Helm should not fail when validating: %s", err)
	}
	// customized labels contain preserved labels of Helm
	labels1 := map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B", "owner": "Helm"}
	if err := validate(labels1); err == nil {
		t.Errorf("Customized labels with preserved labels of Helm must fail when validating")
	}
}

func TestRetrieveCustomizedLabels(t *testing.T) {
	// empty map
	var nilMap map[string]string
	customizedLabels := retrieveCustomizedLabels(nilMap)
	if len(customizedLabels) != 0 {
		t.Errorf("Nil label map retrieved result should be empy map")
	}
	// customized labels with no preserved labels of Helm
	labels0 := map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}
	customizedLabels = retrieveCustomizedLabels(labels0)
	if len(customizedLabels) != len(labels0) {
		t.Errorf("Customized labels with no preserved labels of Helm retrieved result should return the origin labels map")
	}
	// customized labels that every key in preserved labels of Helm
	labels1 := map[string]string{"name": "name", "owner": "helm"}
	customizedLabels = retrieveCustomizedLabels(labels1)
	if len(customizedLabels) != 0 {
		t.Errorf("customized labels that every key in preserved labels of Helm retrieved result should be empy map")
	}
	// customized labels contain preserved labels of Helm
	labels2 := map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B", "owner": "Helm"}
	customizedLabels = retrieveCustomizedLabels(labels2)
	if len(customizedLabels) != 2 {
		t.Errorf("customized labels contain preserved labels of Helm retrieved result should not contain Helm preserved label key")
	}
}
