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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

func TestMemoryName(t *testing.T) {
	mem := NewMemory()
	assert.Equalf(t, MemoryDriverName, mem.Name(), "Expected name to be %q, got %q", MemoryDriverName, mem.Name())
}

func TestMemoryCreate(t *testing.T) {
	var tests = []struct {
		desc string
		rls  *rspb.Release
		err  bool
	}{
		{
			"create should succeed",
			releaseStub("rls-c", 1, "default", common.StatusDeployed),
			false,
		},
		{
			"create should fail (release already exists)",
			releaseStub("rls-a", 1, "default", common.StatusDeployed),
			true,
		},
		{
			"create in namespace should succeed",
			releaseStub("rls-a", 1, "mynamespace", common.StatusDeployed),
			false,
		},
		{
			"create in other namespace should fail (release already exists)",
			releaseStub("rls-c", 1, "mynamespace", common.StatusDeployed),
			true,
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		key := testKey(tt.rls.Name, tt.rls.Version)
		rls := tt.rls
		err := ts.Create(key, rls)

		if tt.err {
			require.Error(t, err, "Did not get expected error for %q\n", tt.desc)
		} else {
			require.NoError(t, err, "failed to create %q", tt.desc)
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
		_, err := ts.Get(tt.key)
		if tt.err {
			require.Error(t, err, "Did not get expected error for %q '%s'\n", tt.desc, tt.key)
		} else {
			require.NoError(t, err, "Failed %q to get '%s'", tt.desc, tt.key)
		}
	}
}

func TestMemoryList(t *testing.T) {
	ts := tsFixtureMemory(t)
	ts.SetNamespace("default")

	// list all deployed releases
	dpl, err := ts.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusDeployed
	})
	// check
	require.NoError(t, err, "Failed to list deployed releases")
	assert.Len(t, dpl, 2, "Expected 2 deployed")

	// list all superseded releases
	ssd, err := ts.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusSuperseded
	})
	// check
	require.NoError(t, err, "Failed to list superseded releases")
	assert.Len(t, ssd, 6, "Expected 6 superseded")

	// list all deleted releases
	del, err := ts.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusUninstalled
	})
	// check
	require.NoError(t, err, "Failed to list deleted releases")
	assert.Empty(t, del, "Expected 0 deleted, got %d", len(del))
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
		require.NoError(t, err, "Failed to query")

		require.Equal(t, len(l), tt.xlen, "Expected %d results, actual %d\n", tt.xlen, len(l))
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
			releaseStub("rls-a", 4, "default", common.StatusSuperseded),
			false,
		},
		{
			"update release does not exist",
			"rls-c.v1",
			releaseStub("rls-c", 1, "default", common.StatusUninstalled),
			true,
		},
		{
			"update release status in namespace",
			"rls-c.v4",
			releaseStub("rls-c", 4, "mynamespace", common.StatusSuperseded),
			false,
		},
		{
			"update release in namespace does not exist",
			"rls-a.v1",
			releaseStub("rls-a", 1, "mynamespace", common.StatusUninstalled),
			true,
		},
	}

	ts := tsFixtureMemory(t)
	for _, tt := range tests {
		err := ts.Update(tt.key, tt.rls)

		if tt.err {
			require.Error(t, err, "Did not get expected error for %q '%s'\n", tt.desc, tt.key)
		} else {
			require.NoError(t, err, "Failed %q", tt.desc)

			ts.SetNamespace(tt.rls.Namespace)

			r, err := ts.Get(tt.key)
			require.NoError(t, err, "Failed to get")
			require.Equalf(t, r, tt.rls, "Expected %v, actual %v\n", tt.rls, r)
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
	require.NoError(t, err, "Query failed")
	startLen := len(start)
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ts.SetNamespace(tt.namespace)

			rel, err := ts.Delete(tt.key)
			if tt.err {
				require.Errorf(t, err, "Did not get expected error for %q '%s'\n", tt.desc, tt.key)
			} else {
				require.NoErrorf(t, err, "Failed %q to get '%s'", tt.desc, tt.key)
				rls := convertReleaserToV1(t, rel)
				require.Equalf(t, tt.key, fmt.Sprintf("%s.v%d", rls.Name, rls.Version), "Asked for delete on %s, but deleted %d", tt.key, rls.Version)
			}
			_, err = ts.Get(tt.key)
			require.Error(t, err, "Expected an error when asking for a deleted key")
		})
	}

	// Make sure that the deleted records are gone.
	ts.SetNamespace("")
	end, err := ts.Query(map[string]string{"status": "deployed"})
	require.NoError(t, err, "Query failed")

	if !assert.Len(t, end, startLen-2) {
		for _, ee := range end {
			rac, err := release.NewAccessor(ee)
			require.NoError(t, err, "unable to get release accessor")
			t.Logf("Name: %s, Version: %d", rac.Name(), rac.Version())
		}
	}
}
