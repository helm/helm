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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"reflect"
	"testing"

	rspb "helm.sh/helm/v3/pkg/release"
)

func TestRecordsAdd(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	var tests = []struct {
		desc string
		key  string
		ok   bool
		rec  *record
	}{
		{
			"add valid key",
			"rls-a.v3",
			false,
			newRecord("rls-a.v3", releaseStub("rls-a", 3, "default", rspb.StatusSuperseded)),
		},
		{
			"add already existing key",
			"rls-a.v1",
			true,
			newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusDeployed)),
		},
	}

	for _, tt := range tests {
		if err := rs.Add(tt.rec); err != nil {
			if !tt.ok {
				t.Fatalf("failed: %q: %s\n", tt.desc, err)
			}
		}
	}
}

func TestRecordsRemove(t *testing.T) {
	var tests = []struct {
		desc string
		key  string
		ok   bool
	}{
		{"remove valid key", "rls-a.v1", false},
		{"remove invalid key", "rls-a.v", true},
		{"remove non-existent key", "rls-z.v1", true},
	}

	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	startLen := rs.Len()

	for _, tt := range tests {
		if r := rs.Remove(tt.key); r == nil {
			if !tt.ok {
				t.Fatalf("Failed to %q (key = %s). Expected nil, got %v",
					tt.desc,
					tt.key,
					r,
				)
			}
		}
	}

	// We expect the total number of records will be less now than there were
	// when we started.
	endLen := rs.Len()
	if endLen >= startLen {
		t.Errorf("expected ending length %d to be less than starting length %d", endLen, startLen)
	}
}

func TestRecordsRemoveAt(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	if len(rs) != 2 {
		t.Fatal("Expected len=2 for mock")
	}

	rs.Remove("rls-a.v1")
	if len(rs) != 1 {
		t.Fatalf("Expected length of rs to be 1, got %d", len(rs))
	}
}

func TestRecordsGet(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	var tests = []struct {
		desc string
		key  string
		rec  *record
	}{
		{
			"get valid key",
			"rls-a.v1",
			newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		},
		{
			"get invalid key",
			"rls-a.v3",
			nil,
		},
	}

	for _, tt := range tests {
		got := rs.Get(tt.key)
		if !reflect.DeepEqual(tt.rec, got) {
			t.Fatalf("Expected %v, got %v", tt.rec, got)
		}
	}
}

func TestRecordsIndex(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	var tests = []struct {
		desc string
		key  string
		sort int
	}{
		{
			"get valid key",
			"rls-a.v1",
			0,
		},
		{
			"get invalid key",
			"rls-a.v3",
			-1,
		},
	}

	for _, tt := range tests {
		got, _ := rs.Index(tt.key)
		if got != tt.sort {
			t.Fatalf("Expected %d, got %d", tt.sort, got)
		}
	}
}

func TestRecordsExists(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	var tests = []struct {
		desc string
		key  string
		ok   bool
	}{
		{
			"get valid key",
			"rls-a.v1",
			true,
		},
		{
			"get invalid key",
			"rls-a.v3",
			false,
		},
	}

	for _, tt := range tests {
		got := rs.Exists(tt.key)
		if got != tt.ok {
			t.Fatalf("Expected %t, got %t", tt.ok, got)
		}
	}
}

func TestRecordsReplace(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
	})

	var tests = []struct {
		desc     string
		key      string
		rec      *record
		expected *record
	}{
		{
			"replace with existing key",
			"rls-a.v2",
			newRecord("rls-a.v3", releaseStub("rls-a", 3, "default", rspb.StatusSuperseded)),
			newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.StatusDeployed)),
		},
		{
			"replace with non existing key",
			"rls-a.v4",
			newRecord("rls-a.v4", releaseStub("rls-a", 4, "default", rspb.StatusDeployed)),
			nil,
		},
	}

	for _, tt := range tests {
		got := rs.Replace(tt.key, tt.rec)
		if !reflect.DeepEqual(tt.expected, got) {
			t.Fatalf("Expected %v, got %v", tt.expected, got)
		}
	}
}
