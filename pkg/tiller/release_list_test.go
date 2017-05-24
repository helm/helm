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

	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
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

	mrs := &mockListServer{}
	if err := rs.ListReleases(&services.ListReleasesRequest{Offset: "", Limit: 64}, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != num {
		t.Errorf("Expected %d releases, got %d", num, len(mrs.val.Releases))
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
		statusCodes []release.Status_Code
		names       []string
	}{
		{
			names:       []string{"kamal"},
			statusCodes: []release.Status_Code{release.Status_DEPLOYED},
		},
		{
			names:       []string{"astrolabe"},
			statusCodes: []release.Status_Code{release.Status_DELETED},
		},
		{
			names:       []string{"kamal", "octant"},
			statusCodes: []release.Status_Code{release.Status_DEPLOYED, release.Status_FAILED},
		},
		{
			names: []string{"kamal", "astrolabe", "octant", "sextant"},
			statusCodes: []release.Status_Code{
				release.Status_DEPLOYED,
				release.Status_DELETED,
				release.Status_FAILED,
				release.Status_UNKNOWN,
			},
		},
	}

	for i, tt := range tests {
		mrs := &mockListServer{}
		if err := rs.ListReleases(&services.ListReleasesRequest{StatusCodes: tt.statusCodes, Offset: "", Limit: 64}, mrs); err != nil {
			t.Fatalf("Failed listing %d: %s", i, err)
		}

		if len(tt.names) != len(mrs.val.Releases) {
			t.Fatalf("Expected %d releases, got %d", len(tt.names), len(mrs.val.Releases))
		}

		for _, name := range tt.names {
			found := false
			for _, rel := range mrs.val.Releases {
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
	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset: "",
		Limit:  int64(limit),
		SortBy: services.ListSort_NAME,
	}
	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != limit {
		t.Errorf("Expected %d releases, got %d", limit, len(mrs.val.Releases))
	}

	for i := 0; i < limit; i++ {
		n := fmt.Sprintf("rel-%d", i+1)
		if mrs.val.Releases[i].Name != n {
			t.Errorf("Expected %q, got %q", n, mrs.val.Releases[i].Name)
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

	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset: "",
		Limit:  64,
		Filter: "neuro[a-z]+",
		SortBy: services.ListSort_NAME,
	}
	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(mrs.val.Releases))
	}

	if mrs.val.Releases[0].Name != "neuroglia" {
		t.Errorf("Unexpected sort order: %v.", mrs.val.Releases)
	}
	if mrs.val.Releases[1].Name != "neuron" {
		t.Errorf("Unexpected sort order: %v.", mrs.val.Releases)
	}
}

func TestReleasesNamespace(t *testing.T) {
	rs := rsFixture()

	names := []string{
		"axon",
		"dendrite",
		"neuron",
		"ribosome",
	}

	namespaces := []string{
		"default",
		"test123",
		"test123",
		"cerebellum",
	}
	num := 4
	for i := 0; i < num; i++ {
		rel := releaseStub()
		rel.Name = names[i]
		rel.Namespace = namespaces[i]
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset:    "",
		Limit:     64,
		Namespace: "test123",
	}

	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(mrs.val.Releases))
	}
}
