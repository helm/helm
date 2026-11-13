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

package util

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	rspb "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/release/common"
)

// note: this test data is shared with filter_test.go.

var releases = []*rspb.Release{
	tsRelease("quiet-bear", 2, 2000, common.StatusSuperseded),
	tsRelease("angry-bird", 4, 3000, common.StatusDeployed),
	tsRelease("happy-cats", 1, 4000, common.StatusUninstalled),
	tsRelease("vocal-dogs", 3, 6000, common.StatusUninstalled),
}

func tsRelease(name string, vers int, dur time.Duration, status common.Status) *rspb.Release {
	info := &rspb.Info{Status: status, LastDeployed: time.Now().Add(dur)}
	return &rspb.Release{
		Name:    name,
		Version: vers,
		Info:    info,
	}
}

func check(t *testing.T, by string, fn func(int, int) bool) {
	t.Helper()
	for i := len(releases) - 1; i > 0; i-- {
		assert.False(t, fn(i, i-1), "release at positions '(%d,%d)' not sorted by %s", i-1, i, by)
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

func TestSortTieBreakers(t *testing.T) {
	early := time.Unix(1700000000, 0)
	late := time.Unix(1700000060, 0)

	key := func(list []*rspb.Release) []string {
		out := make([]string, 0, len(list))
		for _, r := range list {
			out = append(out, fmt.Sprintf("%s:%d", r.Name, r.Version))
		}
		return out
	}
	reversed := func(in []*rspb.Release) []*rspb.Release {
		out := make([]*rspb.Release, 0, len(in))
		for _, r := range slices.Backward(in) {
			out = append(out, r)
		}
		return out
	}
	rotated := func(in []*rspb.Release) []*rspb.Release {
		out := make([]*rspb.Release, 0, len(in))
		out = append(out, in[len(in)-1])
		return append(out, in[:len(in)-1]...)
	}
	rel := func(name string, version int, at time.Time) *rspb.Release {
		return &rspb.Release{Name: name, Version: version, Info: &rspb.Info{LastDeployed: at}}
	}

	tests := []struct {
		name    string
		sortFn  func([]*rspb.Release)
		reverse bool
		input   []*rspb.Release
		want    []string
	}{
		{
			name:   "by name orders the revisions of one release",
			sortFn: SortByName,
			input:  []*rspb.Release{rel("x", 3, late), rel("a", 1, early), rel("x", 1, late), rel("x", 2, late)},
			want:   []string{"a:1", "x:1", "x:2", "x:3"},
		},
		{
			name:   "by revision orders releases that share a revision",
			sortFn: SortByRevision,
			input:  []*rspb.Release{rel("b", 1, late), rel("d", 2, late), rel("a", 1, early), rel("c", 1, late)},
			want:   []string{"a:1", "b:1", "c:1", "d:2"},
		},
		{
			name:   "by date breaks ties on name then revision",
			sortFn: SortByDate,
			input:  []*rspb.Release{rel("y", 3, late), rel("z", 1, early), rel("y", 1, late), rel("y", 2, late)},
			want:   []string{"z:1", "y:1", "y:2", "y:3"},
		},
		{
			name:    "reversed by name",
			sortFn:  SortByName,
			reverse: true,
			input:   []*rspb.Release{rel("x", 3, late), rel("a", 1, early), rel("x", 1, late), rel("x", 2, late)},
			want:    []string{"x:3", "x:2", "x:1", "a:1"},
		},
		{
			name:    "reversed by revision",
			sortFn:  SortByRevision,
			reverse: true,
			input:   []*rspb.Release{rel("b", 1, late), rel("d", 2, late), rel("a", 1, early), rel("c", 1, late)},
			want:    []string{"d:2", "c:1", "b:1", "a:1"},
		},
		{
			name:    "reversed by date",
			sortFn:  SortByDate,
			reverse: true,
			input:   []*rspb.Release{rel("y", 3, late), rel("z", 1, early), rel("y", 1, late), rel("y", 2, late)},
			want:    []string{"y:3", "y:2", "y:1", "z:1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The same set must come out in the same order regardless of the
			// order the releases arrived in.
			for _, order := range [][]*rspb.Release{tt.input, reversed(tt.input), rotated(tt.input), rotated(reversed(tt.input))} {
				list := make([]*rspb.Release, len(order))
				copy(list, order)

				if tt.reverse {
					Reverse(list, tt.sortFn)
				} else {
					tt.sortFn(list)
				}
				assert.Equal(t, tt.want, key(list))
			}
		})
	}
}
