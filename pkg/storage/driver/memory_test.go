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

package driver

import (
	"fmt"
	"reflect"
	"testing"

	rspb "helm.sh/helm/v3/pkg/release"
)

func TestMemoryName(t *testing.T) {
	if mem := NewMemory("default"); mem.Name() != MemoryDriverName {
		t.Errorf("Expected name to be %q, got %q", MemoryDriverName, mem.Name())
	}
}

func TestMemoryCreate(t *testing.T) {
	var tests = []struct {
		desc      string
		namespace string
		rls       *rspb.Release
		err       bool
	}{
		{
			"create should succeed in default namespace",
			"default",
			releaseStub("rls-c", 1, "default", rspb.StatusDeployed),
			false,
		},
		{
			"create should fail (release already exists)",
			"default",
			releaseStub("rls-a", 1, "default", rspb.StatusDeployed),
			true,
		},
		{
			"create should succeed in testing namespace",
			"testing",
			releaseStub("rls-c", 1, "default", rspb.StatusDeployed),
			false,
		},
		{
			"create should fail (release already exists) in testing namespace",
			"testing",
			releaseStub("rls-c", 1, "default", rspb.StatusDeployed),
			true,
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
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
	ts := tsFixtureMemory(t)
	var tests = []struct {
		desc      string
		namespace string
		key       string
		err       bool
	}{
		{"release key should exist in default namespace", "default", "rls-a.v1", false},
		{"release key should not exist in default namespace", "default", "rls-a.v5", true},
		{"release key should exist in testing namespace", "testing", "rls-a.v1", false},
		{"release key should not exist in testing namespace", "testing", "rls-a.v5", true},
	}

	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
		if _, err := ts.Get(tt.key); err != nil {
			if !tt.err {
				t.Fatalf("Failed %q to get '%s': %q\n", tt.desc, tt.key, err)
			}
		}
	}
}

func TestMemoryQuery(t *testing.T) {
	ts := tsFixtureMemory(t)

	var tests = []struct {
		desc      string
		namespace string
		xlen      int
		lbs       map[string]string
	}{
		{
			"should be 2 query results for default namespace",
			"default",
			2,
			map[string]string{"status": "deployed"},
		},
		{
			"should be 1 query result for testing namespace",
			"testing",
			1,
			map[string]string{"status": "deployed"},
		},
		{
			"should be 3 query results for all namespaces",
			"",
			3,
			map[string]string{"status": "deployed"},
		},
	}

	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
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
	ts := tsFixtureMemory(t)

	var tests = []struct {
		desc      string
		namespace string
		key       string
		rls       *rspb.Release
		err       bool
	}{
		{
			"update release status which exists in default namespace",
			"default",
			"rls-a.v4",
			releaseStub("rls-a", 4, "default", rspb.StatusSuperseded),
			false,
		},
		{
			"update release status which exists in testing namespace",
			"testing",
			"rls-a.v1",
			releaseStub("rls-a", 1, "default", rspb.StatusSuperseded),
			false,
		},
		{
			"update release does not exist",
			"default",
			"rls-z.v1",
			releaseStub("rls-z", 1, "default", rspb.StatusUninstalled),
			true,
		},
		{
			"update release does not exist",
			"testing",
			"rls-z.v1",
			releaseStub("rls-z", 1, "default", rspb.StatusUninstalled),
			true,
		},
	}

	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
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
			t.Fatalf("Expected %v, actual %v\n", tt.rls, r)
		}
	}
}

func TestMemoryDelete(t *testing.T) {
	var tests = []struct {
		desc      string
		namespace string
		key       string
		err       bool
	}{
		{"release key should exist in default namespace", "default", "rls-a.v1", false},
		{"release key should not exist in default namespace", "default", "rls-a.v5", true},
		{"release key should exist in testing namespace", "testing", "rls-a.v1", false},
		{"release key should not exist in testing namespace", "testing", "rls-a.v5", true},
	}

	ts := tsFixtureMemory(t)
	// all namespaces
	ts.namespace = ""
	start, err := ts.Query(map[string]string{"name": "rls-a"})
	if err != nil {
		t.Errorf("Query failed: %s", err)
	}
	startLen := len(start)
	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
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

	// all namespaces
	ts.namespace = ""
	// Make sure that the deleted records are gone.
	end, err := ts.Query(map[string]string{"name": "rls-a"})
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
