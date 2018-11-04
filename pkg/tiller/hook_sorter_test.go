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

package tiller

import (
	"sort"
	"testing"

	"k8s.io/helm/pkg/hapi/release"
)

func TestHookSorter(t *testing.T) {
	hooks := []*release.Hook{
		{
			Name:   "g",
			Kind:   "pre-install",
			Weight: 99,
		},
		{
			Name:   "f",
			Kind:   "pre-install",
			Weight: 3,
		},
		{
			Name:   "b",
			Kind:   "pre-install",
			Weight: -3,
		},
		{
			Name:   "e",
			Kind:   "pre-install",
			Weight: 3,
		},
		{
			Name:   "a",
			Kind:   "pre-install",
			Weight: -10,
		},
		{
			Name:   "c",
			Kind:   "pre-install",
			Weight: 0,
		},
		{
			Name:   "d",
			Kind:   "pre-install",
			Weight: 3,
		},
	}

	sort.Sort(hookByWeight(hooks))
	got := ""
	expect := "abcdefg"
	for _, r := range hooks {
		got += r.Name
	}
	if got != expect {
		t.Errorf("Expected %q, got %q", expect, got)
	}
}
