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

package releaseutil // import "k8s.io/helm/pkg/releaseutil"

import (
	"testing"
	"time"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/timeconv"
)

// note: this test data is shared with filter_test.go.

var releases = []*rspb.Release{
	tsRelease("quiet-bear", 2, 2000, rspb.Status_SUPERSEDED),
	tsRelease("angry-bird", 4, 3000, rspb.Status_DEPLOYED),
	tsRelease("happy-cats", 1, 4000, rspb.Status_DELETED),
	tsRelease("vocal-dogs", 3, 6000, rspb.Status_DELETED),
}

func tsRelease(name string, vers int32, dur time.Duration, code rspb.Status_Code) *rspb.Release {
	tmsp := timeconv.Timestamp(time.Now().Add(time.Duration(dur)))
	info := &rspb.Info{Status: &rspb.Status{Code: code}, LastDeployed: tmsp}
	return &rspb.Release{
		Name:    name,
		Version: vers,
		Info:    info,
	}
}

func check(t *testing.T, by string, fn func(int, int) bool) {
	for i := len(releases) - 1; i > 0; i-- {
		if fn(i, i-1) {
			t.Errorf("release at positions '(%d,%d)' not sorted by %s", i-1, i, by)
		}
	}
}

func TestSortByName(t *testing.T) {
	SortByName(releases)

	check(t, "ByName", func(i, j int) bool {
		ni := releases[i].Name
		nj := releases[j].Name
		return ni < nj
	})
}

func TestSortByDate(t *testing.T) {
	SortByDate(releases)

	check(t, "ByDate", func(i, j int) bool {
		ti := releases[i].Info.LastDeployed.Seconds
		tj := releases[j].Info.LastDeployed.Seconds
		return ti < tj
	})
}

func TestSortByRevision(t *testing.T) {
	SortByRevision(releases)

	check(t, "ByRevision", func(i, j int) bool {
		vi := releases[i].Version
		vj := releases[j].Version
		return vi < vj
	})
}
