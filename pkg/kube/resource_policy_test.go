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

package kube

import "testing"

func TestResourcePolicyIsKeep(t *testing.T) {
	type annotations map[string]string
	type testcase struct {
		annotations
		keep bool
	}
	cases := []testcase{
		{nil, false},
		{
			annotations{
				"foo": "bar",
			},
			false,
		},
		{
			annotations{
				ResourcePolicyAnno: "keep",
			},
			true,
		},
		{
			annotations{
				ResourcePolicyAnno: "KEEP   ",
			},
			true,
		},
		{
			annotations{
				ResourcePolicyAnno: "",
			},
			true,
		},
		{
			annotations{
				ResourcePolicyAnno: "delete",
			},
			false,
		},
		{
			annotations{
				ResourcePolicyAnno: "DELETE",
			},
			true,
		},
	}

	for _, tc := range cases {
		if tc.keep != ResourcePolicyIsKeep(tc.annotations) {
			t.Errorf("Expected function to return %t for annotations %v", tc.keep, tc.annotations)
		}
	}
}
