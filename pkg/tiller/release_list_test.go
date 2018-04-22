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

package tiller

import (
	"fmt"
	"testing"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

func TestListReleases(t *testing.T) {
	rs := rsFixture()
	num := 7
	for i := 0; i < num; i++ {
		rel := releaseStub()
		rel.Name = fmt.Sprintf("rel-%d", i)
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	rels, err := rs.ListReleases(&hapi.ListReleasesRequest{})
	if err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(rels) != num {
		t.Errorf("Expected %d releases, got %d", num, len(rels))
	}
}

func TestListReleasesByStatus(t *testing.T) {
	rs := rsFixture()
	stubs := []*release.Release{
		namedReleaseStub("kamal", release.Status_DEPLOYED),
		namedReleaseStub("astrolabe", release.Status_DELETED),
		namedReleaseStub("octant", release.Status_FAILED),
		namedReleaseStub("sextant", release.Status_UNKNOWN),
	}
	for _, stub := range stubs {
		if err := rs.env.Releases.Create(stub); err != nil {
			t.Fatalf("Could not create stub: %s", err)
		}
	}

	tests := []struct {
		statusCodes []release.StatusCode
		names       []string
	}{
		{
			names:       []string{"kamal"},
			statusCodes: []release.StatusCode{release.Status_DEPLOYED},
		},
		{
			names:       []string{"astrolabe"},
			statusCodes: []release.StatusCode{release.Status_DELETED},
		},
		{
			names:       []string{"kamal", "octant"},
			statusCodes: []release.StatusCode{release.Status_DEPLOYED, release.Status_FAILED},
		},
		{
			names: []string{"kamal", "astrolabe", "octant", "sextant"},
			statusCodes: []release.StatusCode{
				release.Status_DEPLOYED,
				release.Status_DELETED,
				release.Status_FAILED,
				release.Status_UNKNOWN,
			},
		},
	}

	for i, tt := range tests {
		rels, err := rs.ListReleases(&hapi.ListReleasesRequest{StatusCodes: tt.statusCodes, Offset: "", Limit: 64})
		if err != nil {
			t.Fatalf("Failed listing %d: %s", i, err)
		}

		if len(tt.names) != len(rels) {
			t.Fatalf("Expected %d releases, got %d", len(tt.names), len(rels))
		}

		for _, name := range tt.names {
			found := false
			for _, rel := range rels {
				if rel.Name == name {
					found = true
				}
			}
			if !found {
				t.Errorf("%d: Did not find name %q", i, name)
			}
		}
	}
}

func TestListReleasesSort(t *testing.T) {
	rs := rsFixture()

	// Put them in by reverse order so that the mock doesn't "accidentally"
	// sort.
	num := 7
	for i := num; i > 0; i-- {
		rel := releaseStub()
		rel.Name = fmt.Sprintf("rel-%d", i)
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	limit := 6
	req := &hapi.ListReleasesRequest{
		Offset: "",
		Limit:  int64(limit),
		SortBy: hapi.ListSort_NAME,
	}
	rels, err := rs.ListReleases(req)
	if err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	// if len(rels) != limit {
	// 	t.Errorf("Expected %d releases, got %d", limit, len(rels))
	// }

	for i := 0; i < limit; i++ {
		n := fmt.Sprintf("rel-%d", i+1)
		if rels[i].Name != n {
			t.Errorf("Expected %q, got %q", n, rels[i].Name)
		}
	}
}

func TestListReleasesFilter(t *testing.T) {
	rs := rsFixture()
	names := []string{
		"axon",
		"dendrite",
		"neuron",
		"neuroglia",
		"synapse",
		"nucleus",
		"organelles",
	}
	num := 7
	for i := 0; i < num; i++ {
		rel := releaseStub()
		rel.Name = names[i]
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	req := &hapi.ListReleasesRequest{
		Offset: "",
		Limit:  64,
		Filter: "neuro[a-z]+",
		SortBy: hapi.ListSort_NAME,
	}
	rels, err := rs.ListReleases(req)
	if err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(rels) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(rels))
	}

	if rels[0].Name != "neuroglia" {
		t.Errorf("Unexpected sort order: %v.", rels)
	}
	if rels[1].Name != "neuron" {
		t.Errorf("Unexpected sort order: %v.", rels)
	}
}
