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

package driver // import "k8s.io/helm/pkg/storage/driver"

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
