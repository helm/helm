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

package driver

import (
	"fmt"
	"reflect"
	"testing"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestMemoryName(t *testing.T) {
	if mem := NewMemory(); mem.Name() != MemoryDriverName {
		t.Errorf("Expected name to be %q, got %q", MemoryDriverName, mem.Name())
	}
}

func TestMemoryCreate(t *testing.T) {
	var tests = []struct {
		desc string
		rls  *rspb.Release
		err  bool
	}{
		{
			"create should success",
			releaseStub("rls-c", 1, "default", rspb.Status_DEPLOYED),
			false,
		},
		{
			"create should fail (release already exists)",
			releaseStub("rls-a", 1, "default", rspb.Status_DEPLOYED),
			true,
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		key := testKey(tt.rls.Name, tt.rls.Version)
		rls := tt.rls

		if err := ts.Create(key, rls); err != nil {
			if !tt.err {
				t.Fatalf("failed to create %q: %s", tt.desc, err)
			}
		}
	}
}

func TestMemoryGet(t *testing.T) {
	var tests = []struct {
		desc string
		key  string
		err  bool
	}{
		{"release key should exist", "rls-a.v1", false},
		{"release key should not exist", "rls-a.v5", true},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		if _, err := ts.Get(tt.key); err != nil {
			if !tt.err {
				t.Fatalf("Failed %q to get '%s': %q\n", tt.desc, tt.key, err)
			}
		}
	}
}

func TestMemoryQuery(t *testing.T) {
	var tests = []struct {
		desc string
		xlen int
		lbs  map[string]string
	}{
		{
			"should be 2 query results",
			2,
			map[string]string{"STATUS": "DEPLOYED"},
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		l, err := ts.Query(tt.lbs)
		if err != nil {
			t.Fatalf("Failed to query: %s\n", err)
		}

		if tt.xlen != len(l) {
			t.Fatalf("Expected %d results, actual %d\n", tt.xlen, len(l))
		}
	}
}

func TestMemoryUpdate(t *testing.T) {
	var tests = []struct {
		desc string
		key  string
		rls  *rspb.Release
		err  bool
	}{
		{
			"update release status",
			"rls-a.v4",
			releaseStub("rls-a", 4, "default", rspb.Status_SUPERSEDED),
			false,
		},
		{
			"update release does not exist",
			"rls-z.v1",
			releaseStub("rls-z", 1, "default", rspb.Status_DELETED),
			true,
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		if err := ts.Update(tt.key, tt.rls); err != nil {
			if !tt.err {
				t.Fatalf("Failed %q: %s\n", tt.desc, err)
			}
			continue
		}

		r, err := ts.Get(tt.key)
		if err != nil {
			t.Fatalf("Failed to get: %s\n", err)
		}

		if !reflect.DeepEqual(r, tt.rls) {
			t.Fatalf("Expected %s, actual %s\n", tt.rls, r)
		}
	}
}

func TestMemoryDelete(t *testing.T) {
	var tests = []struct {
		desc string
		key  string
		err  bool
	}{
		{"release key should exist", "rls-a.v1", false},
		{"release key should not exist", "rls-a.v5", true},
	}

	ts := tsFixtureMemory(t)
	start, err := ts.Query(map[string]string{"NAME": "rls-a"})
	if err != nil {
		t.Errorf("Query failed: %s", err)
	}
	startLen := len(start)
	for _, tt := range tests {
		if rel, err := ts.Delete(tt.key); err != nil {
			if !tt.err {
				t.Fatalf("Failed %q to get '%s': %q\n", tt.desc, tt.key, err)
			}
			continue
		} else if fmt.Sprintf("%s.v%d", rel.Name, rel.Version) != tt.key {
			t.Fatalf("Asked for delete on %s, but deleted %d", tt.key, rel.Version)
		}
		_, err := ts.Get(tt.key)
		if err == nil {
			t.Errorf("Expected an error when asking for a deleted key")
		}
	}

	// Make sure that the deleted records are gone.
	end, err := ts.Query(map[string]string{"NAME": "rls-a"})
	if err != nil {
		t.Errorf("Query failed: %s", err)
	}
	endLen := len(end)

	if startLen <= endLen {
		t.Errorf("expected start %d to be less than end %d", startLen, endLen)
		for _, ee := range end {
			t.Logf("Name: %s, Version: %d", ee.Name, ee.Version)
		}
	}

}
