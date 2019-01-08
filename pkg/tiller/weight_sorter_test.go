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
	"bytes"
	"testing"

	"k8s.io/helm/pkg/manifest"
)

func TestWeightSorter(t *testing.T) {
	manifests := []Manifest{
		{
			Name: "a",
			Weight: &manifest.Weight{
				Chart:    1,
				Manifest: 5,
			},
		},
		{
			Name: "u",
			Weight: &manifest.Weight{
				Chart:    0,
				Manifest: 2,
			},
		},
		{
			Name: "d",
			Weight: &manifest.Weight{
				Chart:    1,
				Manifest: 0,
			},
		},
		{
			Name: "e",
			Weight: &manifest.Weight{
				Chart:    2,
				Manifest: 0,
			},
		},
		{
			Name: "t",
			Weight: &manifest.Weight{
				Chart:    10,
				Manifest: 4294967295,
			},
		},
		{
			Name: "b",
			Weight: &manifest.Weight{
				Chart:    0,
				Manifest: 0,
			},
		},
		{
			Name: "p",
			Weight: &manifest.Weight{
				Chart:    1,
				Manifest: 5,
			},
		},
		{
			Name: "s",
			Weight: &manifest.Weight{
				Chart:    10,
				Manifest: 10,
			},
		},
	}

	for _, test := range []struct {
		description string
		order       SortType
		expected    string
	}{
		{"install", SortInstall, "tseapdub"},
		{"uninstall", SortUninstall, "budapest"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(manifests); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
			defer buf.Reset()
			for _, r := range sortByWeight(manifests, test.order) {
				buf.WriteString(r.Name)
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}
