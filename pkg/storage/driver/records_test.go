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

package driver // import "k8s.io/helm/pkg/storage/driver"

import (
	"testing"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestRecordsAdd(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.Status_SUPERSEDED)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.Status_DEPLOYED)),
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
			newRecord("rls-a.v3", releaseStub("rls-a", 3, "default", rspb.Status_SUPERSEDED)),
		},
		{
			"add already existing key",
			"rls-a.v1",
			true,
			newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.Status_DEPLOYED)),
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
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.Status_SUPERSEDED)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.Status_DEPLOYED)),
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
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", rspb.Status_SUPERSEDED)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", rspb.Status_DEPLOYED)),
	})

	if len(rs) != 2 {
		t.Fatal("Expected len=2 for mock")
	}

	rs.Remove("rls-a.v1")
	if len(rs) != 1 {
		t.Fatalf("Expected length of rs to be 1, got %d", len(rs))
	}
}
