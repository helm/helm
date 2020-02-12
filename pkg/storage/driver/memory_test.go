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
			"create should succeed",
			releaseStub("rls-c", 1, "default", rspb.StatusDeployed),
			false,
		},
		{
			"create should fail (release already exists)",
			releaseStub("rls-a", 1, "default", rspb.StatusDeployed),
			true,
		},
		{
			"create in namespace should succeed",
			releaseStub("rls-a", 1, "mynamespace", rspb.StatusDeployed),
			false,
		},
		{
			"create in other namespace should fail (release already exists)",
			releaseStub("rls-c", 1, "mynamespace", rspb.StatusDeployed),
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
		} else if tt.err {
			t.Fatalf("Did not get expected error for %q\n", tt.desc)
		}
	}
}

func TestMemoryGet(t *testing.T) {
	var tests = []struct {
		desc      string
		key       string
		namespace string
		err       bool
	}{
		{"release key should exist", "rls-a.v1", "default", false},
		{"release key should not exist", "rls-a.v5", "default", true},
		{"release key in namespace should exist", "rls-c.v1", "mynamespace", false},
		{"release key in namespace should not exist", "rls-a.v1", "mynamespace", true},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		ts.SetNamespace(tt.namespace)
		if _, err := ts.Get(tt.key); err != nil {
			if !tt.err {
				t.Fatalf("Failed %q to get '%s': %q\n", tt.desc, tt.key, err)
			}
		} else if tt.err {
			t.Fatalf("Did not get expected error for %q '%s'\n", tt.desc, tt.key)
		}
	}
}

func TestMemoryList(t *testing.T) {
	ts := tsFixtureMemory(t)
	ts.SetNamespace("default")

	// list all deployed releases
	dpl, err := ts.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusDeployed
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed releases: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}

	// list all superseded releases
	ssd, err := ts.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusSuperseded
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded releases: %s", err)
	}
	if len(ssd) != 6 {
		t.Errorf("Expected 6 superseded, got %d", len(ssd))
	}

	// list all deleted releases
	del, err := ts.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusUninstalled
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted releases: %s", err)
	}
	if len(del) != 0 {
		t.Errorf("Expected 0 deleted, got %d", len(del))
	}
}

func TestMemoryQuery(t *testing.T) {
	var tests = []struct {
		desc      string
		xlen      int
		namespace string
		lbs       map[string]string
	}{
		{
			"should be 2 query results",
			2,
			"default",
			map[string]string{"status": "deployed"},
		},
		{
			"should be 1 query result",
			1,
			"mynamespace",
			map[string]string{"status": "deployed"},
		},
	}

	ts := tsFixtureMemory(t)
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
	var tests = []struct {
		desc string
		key  string
		rls  *rspb.Release
		err  bool
	}{
		{
			"update release status",
			"rls-a.v4",
			releaseStub("rls-a", 4, "default", rspb.StatusSuperseded),
			false,
		},
		{
			"update release does not exist",
			"rls-c.v1",
			releaseStub("rls-c", 1, "default", rspb.StatusUninstalled),
			true,
		},
		{
			"update release status in namespace",
			"rls-c.v4",
			releaseStub("rls-c", 4, "mynamespace", rspb.StatusSuperseded),
			false,
		},
		{
			"update release in namespace does not exist",
			"rls-a.v1",
			releaseStub("rls-a", 1, "mynamespace", rspb.StatusUninstalled),
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
		} else if tt.err {
			t.Fatalf("Did not get expected error for %q '%s'\n", tt.desc, tt.key)
		}

		ts.SetNamespace(tt.rls.Namespace)
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
		key       string
		namespace string
		err       bool
	}{
		{"release key should exist", "rls-a.v4", "default", false},
		{"release key should not exist", "rls-a.v5", "default", true},
		{"release key from other namespace should not exist", "rls-c.v4", "default", true},
		{"release key from namespace should exist", "rls-c.v4", "mynamespace", false},
		{"release key from namespace should not exist", "rls-c.v5", "mynamespace", true},
		{"release key from namespace2 should not exist", "rls-a.v4", "mynamespace", true},
	}

	ts := tsFixtureMemory(t)
	ts.SetNamespace("")
	start, err := ts.Query(map[string]string{"status": "deployed"})
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
		} else if tt.err {
			t.Fatalf("Did not get expected error for %q '%s'\n", tt.desc, tt.key)
		} else if fmt.Sprintf("%s.v%d", rel.Name, rel.Version) != tt.key {
			t.Fatalf("Asked for delete on %s, but deleted %d", tt.key, rel.Version)
		}
		_, err := ts.Get(tt.key)
		if err == nil {
			t.Errorf("Expected an error when asking for a deleted key")
		}
	}

	// Make sure that the deleted records are gone.
	ts.SetNamespace("")
	end, err := ts.Query(map[string]string{"status": "deployed"})
	if err != nil {
		t.Errorf("Query failed: %s", err)
	}
	endLen := len(end)

	if startLen-2 != endLen {
		t.Errorf("expected end to be %d instead of %d", startLen-2, endLen)
		for _, ee := range end {
			t.Logf("Name: %s, Version: %d", ee.Name, ee.Version)
		}
	}

}
