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

package action

import (
	"sort"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestHookByWeight(t *testing.T) {
	hooks := []*release.Hook{
		// The previous sorter sorts by (Weight, Name) so would fail this test as
		// "PodA" < "ServiceAccountA".
		{Weight: 1, Kind: "Pod", Name: "PodA"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountB"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountA"},
		{Weight: 0, Kind: "Job", Name: "Job"},
		{Weight: 0, Kind: "UnknownB", Name: "UnknownB1"},
		{Weight: 0, Kind: "UnknownB", Name: "UnknownB0"},
		{Weight: 0, Kind: "UnknownA", Name: "UnknownA"},
		{Weight: -1, Kind: "APIService", Name: "APIServiceB"},
		{Weight: -1, Kind: "APIService", Name: "APIServiceA"},
	}

	sort.Stable(hookByWeight(hooks))

	expected := []*release.Hook{
		{Weight: -1, Kind: "APIService", Name: "APIServiceA"},
		{Weight: -1, Kind: "APIService", Name: "APIServiceB"},
		{Weight: 0, Kind: "Job", Name: "Job"},
		{Weight: 0, Kind: "UnknownA", Name: "UnknownA"},
		{Weight: 0, Kind: "UnknownB", Name: "UnknownB0"},
		{Weight: 0, Kind: "UnknownB", Name: "UnknownB1"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountA"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountB"},
		{Weight: 1, Kind: "Pod", Name: "PodA"},
	}

	for i, hook := range hooks {
		if hook.Name != expected[i].Name {
			t.Errorf("Expected hook %d to be %s, got %s", i, expected[i].Name, hook.Name)
		}
	}
}

func TestHookByWeight_KindSorted(t *testing.T) {
	// This test assumes that the list of hooks starts off sorted by Kind.
	hooks := []*release.Hook{
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountB"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountA"},
		{Weight: 0, Kind: "Pod", Name: "podB"},
		{Weight: -1, Kind: "Pod", Name: "podA"},
		{Weight: 0, Kind: "Job", Name: "Job"},
		{Weight: 1, Kind: "APIService", Name: "APIServiceA"},
		{Weight: 0, Kind: "UnknownA", Name: "Unknown1"},
		{Weight: 0, Kind: "UnknownB", Name: "Unknown0"},
	}

	sort.Stable(hookByWeight(hooks))

	expected := []*release.Hook{
		{Weight: -1, Kind: "Pod", Name: "podA"},
		{Weight: 0, Kind: "Pod", Name: "podB"},
		{Weight: 0, Kind: "Job", Name: "Job"},
		{Weight: 0, Kind: "UnknownA", Name: "Unknown1"},
		{Weight: 0, Kind: "UnknownB", Name: "Unknown0"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountA"},
		{Weight: 1, Kind: "ServiceAccount", Name: "ServiceAccountB"},
		{Weight: 1, Kind: "APIService", Name: "APIServiceA"},
	}

	for i, hook := range hooks {
		if hook.Name != expected[i].Name {
			t.Errorf("Expected hook %d to be %s, got %s", i, expected[i].Name, hook.Name)
		}
	}
}
