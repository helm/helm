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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/release/common"
)

func TestRecordsAdd(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
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
			newRecord("rls-a.v3", releaseStub("rls-a", 3, "default", common.StatusSuperseded)),
		},
		{
			"add already existing key",
			"rls-a.v1",
			true,
			newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusDeployed)),
		},
	}

	for _, tt := range tests {
		err := rs.Add(tt.rec)
		if !tt.ok {
			require.NoError(t, err, "failed: %q:", tt.desc)
		} else {
			require.Error(t, err)
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
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
	})

	startLen := rs.Len()

	for _, tt := range tests {
		r := rs.Remove(tt.key)
		if tt.ok {
			require.Nil(t, r, "Failed to %q (key = %s). Expected nil, got %v", tt.desc, tt.key, r)
		} else {
			require.NotNil(t, r)
		}
	}

	// We expect the total number of records will be less now than there were
	// when we started.
	endLen := rs.Len()
	assert.Lessf(t, endLen, startLen, "expected ending length %d to be less than starting length %d", endLen, startLen)
}

func TestRecordsRemoveAt(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
	})

	require.Len(t, rs, 2, "Expected len=2 for mock")

	rs.Remove("rls-a.v1")
	require.Len(t, rs, 1, "Expected length of rs to be 1, got %d", len(rs))
}

func TestRecordsGet(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
	})

	var tests = []struct {
		desc string
		key  string
		rec  *record
	}{
		{
			"get valid key",
			"rls-a.v1",
			newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		},
		{
			"get invalid key",
			"rls-a.v3",
			nil,
		},
	}

	for _, tt := range tests {
		got := rs.Get(tt.key)
		require.Equal(t, tt.rec, got, "Expected %v, got %v", tt.rec, got)
	}
}

func TestRecordsIndex(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
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
		require.Equal(t, tt.sort, got, "Expected %d, got %d", tt.sort, got)
	}
}

func TestRecordsExists(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
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
		require.Equal(t, tt.ok, got, "Expected %t, got %t", tt.ok, got)
	}
}

func TestRecordsReplace(t *testing.T) {
	rs := records([]*record{
		newRecord("rls-a.v1", releaseStub("rls-a", 1, "default", common.StatusSuperseded)),
		newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
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
			newRecord("rls-a.v3", releaseStub("rls-a", 3, "default", common.StatusSuperseded)),
			newRecord("rls-a.v2", releaseStub("rls-a", 2, "default", common.StatusDeployed)),
		},
		{
			"replace with non existing key",
			"rls-a.v4",
			newRecord("rls-a.v4", releaseStub("rls-a", 4, "default", common.StatusDeployed)),
			nil,
		},
	}

	for _, tt := range tests {
		got := rs.Replace(tt.key, tt.rec)
		require.Equalf(t, tt.expected, got, "Expected %v, got %v", tt.expected, got)
	}
}
