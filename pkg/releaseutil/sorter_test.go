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

package releaseutil // import "helm.sh/helm/v3/pkg/releaseutil"

import (
	"testing"
	"time"

	rspb "helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

// note: this test data is shared with filter_test.go.

var releases = []*rspb.Release{
	tsRelease("quiet-bear", 2, 2000, rspb.StatusSuperseded),
	tsRelease("angry-bird", 4, 3000, rspb.StatusDeployed),
	tsRelease("happy-cats", 1, 4000, rspb.StatusUninstalled),
	tsRelease("vocal-dogs", 3, 6000, rspb.StatusUninstalled),
}

func tsRelease(name string, vers int, dur time.Duration, status rspb.Status) *rspb.Release {
	info := &rspb.Info{Status: status, LastDeployed: helmtime.Now().Add(dur)}
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
		ti := releases[i].Info.LastDeployed.Second()
		tj := releases[j].Info.LastDeployed.Second()
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

func TestReverseSortByName(t *testing.T) {
	Reverse(releases, SortByName)
	check(t, "ByName", func(i, j int) bool {
		ni := releases[i].Name
		nj := releases[j].Name
		return ni > nj
	})
}

func TestReverseSortByDate(t *testing.T) {
	Reverse(releases, SortByDate)
	check(t, "ByDate", func(i, j int) bool {
		ti := releases[i].Info.LastDeployed.Second()
		tj := releases[j].Info.LastDeployed.Second()
		return ti > tj
	})
}

func TestReverseSortByRevision(t *testing.T) {
	Reverse(releases, SortByRevision)
	check(t, "ByRevision", func(i, j int) bool {
		vi := releases[i].Version
		vj := releases[j].Version
		return vi > vj
	})
}
