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

package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/repo/v1"
)

func TestSortScore(t *testing.T) {
	in := []*Result{
		{Name: "bbb", Score: 0, Chart: &repo.ChartVersion{Metadata: &chart.Metadata{Version: "1.2.3"}}},
		{Name: "aaa", Score: 5},
		{Name: "abb", Score: 5},
		{Name: "aab", Score: 0},
		{Name: "bab", Score: 5},
		{Name: "ver", Score: 5, Chart: &repo.ChartVersion{Metadata: &chart.Metadata{Version: "1.2.4"}}},
		{Name: "ver", Score: 5, Chart: &repo.ChartVersion{Metadata: &chart.Metadata{Version: "1.2.3"}}},
	}
	expect := []string{"aab", "bbb", "aaa", "abb", "bab", "ver", "ver"}
	expectScore := []int{0, 0, 5, 5, 5, 5, 5}
	SortScore(in)

	// Test Score
	for i := range expectScore {
		assert.Equalf(t, expectScore[i], in[i].Score, "Sort error on index %d: expected %d, got %d", i, expectScore[i], in[i].Score)
	}
	// Test Name
	for i := range expect {
		assert.Equalf(t, expect[i], in[i].Name, "Sort error: expected %s, got %s", expect[i], in[i].Name)
	}

	// Test version of last two items
	assert.Equalf(t, "1.2.4", in[5].Chart.Version, "Expected 1.2.4, got %s", in[5].Chart.Version)
	assert.Equal(t, "1.2.3", in[6].Chart.Version, "Expected 1.2.3 to be last")
}

var indexfileEntries = map[string]repo.ChartVersions{
	"niña": {
		{
			URLs: []string{"http://example.com/charts/nina-0.1.0.tgz"},
			Metadata: &chart.Metadata{
				Name:        "niña",
				Version:     "0.1.0",
				Description: "One boat",
			},
		},
	},
	"pinta": {
		{
			URLs: []string{"http://example.com/charts/pinta-0.1.0.tgz"},
			Metadata: &chart.Metadata{
				Name:        "pinta",
				Version:     "0.1.0",
				Description: "Two ship",
			},
		},
	},
	"santa-maria": {
		{
			URLs: []string{"http://example.com/charts/santa-maria-1.2.3.tgz"},
			Metadata: &chart.Metadata{
				Name:        "santa-maria",
				Version:     "1.2.3",
				Description: "Three boat",
			},
		},
		{
			URLs: []string{"http://example.com/charts/santa-maria-1.2.2-rc-1.tgz"},
			Metadata: &chart.Metadata{
				Name:        "santa-maria",
				Version:     "1.2.2-RC-1",
				Description: "Three boat",
			},
		},
	},
}

func loadTestIndex(_ *testing.T, all bool) *Index {
	i := NewIndex()
	i.AddRepo("testing", &repo.IndexFile{Entries: indexfileEntries}, all)
	i.AddRepo("ztesting", &repo.IndexFile{Entries: map[string]repo.ChartVersions{
		"Pinta": {
			{
				URLs: []string{"http://example.com/charts/pinta-2.0.0.tgz"},
				Metadata: &chart.Metadata{
					Name:        "Pinta",
					Version:     "2.0.0",
					Description: "Two ship, version two",
				},
			},
		},
	}}, all)
	return i
}

func TestAll(t *testing.T) {
	i := loadTestIndex(t, false)
	all := i.All()
	assert.Lenf(t, all, 4, "Expected 4 entries, got %d", len(all))

	i = loadTestIndex(t, true)
	all = i.All()
	assert.Lenf(t, all, 5, "Expected 5 entries, got %d", len(all))
}

func TestAddRepo_Sort(t *testing.T) {
	i := loadTestIndex(t, true)
	sr, err := i.Search("TESTING/SANTA-MARIA", 100, false)
	require.NoError(t, err)
	SortScore(sr)

	ch := sr[0]
	expect := "1.2.3"
	assert.Equalf(t, ch.Chart.Version, expect, "Expected %q, got %q", expect, ch.Chart.Version)
}

func TestSearchByName(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		expect  []*Result
		regexp  bool
		fail    bool
		failMsg string
	}{
		{
			name:  "basic search for one result",
			query: "santa-maria",
			expect: []*Result{
				{Name: "testing/santa-maria"},
			},
		},
		{
			name:  "basic search for two results",
			query: "pinta",
			expect: []*Result{
				{Name: "testing/pinta"},
				{Name: "ztesting/Pinta"},
			},
		},
		{
			name:  "repo-specific search for one result",
			query: "ztesting/pinta",
			expect: []*Result{
				{Name: "ztesting/Pinta"},
			},
		},
		{
			name:  "partial name search",
			query: "santa",
			expect: []*Result{
				{Name: "testing/santa-maria"},
			},
		},
		{
			name:  "description search, one result",
			query: "Three",
			expect: []*Result{
				{Name: "testing/santa-maria"},
			},
		},
		{
			name:  "description search, two results",
			query: "two",
			expect: []*Result{
				{Name: "testing/pinta"},
				{Name: "ztesting/Pinta"},
			},
		},
		{
			name:  "search mixedCase and result should be mixedCase too",
			query: "pinta",
			expect: []*Result{
				{Name: "testing/pinta"},
				{Name: "ztesting/Pinta"},
			},
		},
		{
			name:  "description upper search, two results",
			query: "TWO",
			expect: []*Result{
				{Name: "testing/pinta"},
				{Name: "ztesting/Pinta"},
			},
		},
		{
			name:   "nothing found",
			query:  "mayflower",
			expect: []*Result{},
		},
		{
			name:  "regexp, one result",
			query: "Th[ref]*",
			expect: []*Result{
				{Name: "testing/santa-maria"},
			},
			regexp: true,
		},
		{
			name:    "regexp, fail compile",
			query:   "th[",
			expect:  []*Result{},
			regexp:  true,
			fail:    true,
			failMsg: "error parsing regexp:",
		},
	}

	i := loadTestIndex(t, false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charts, err := i.Search(tt.query, 100, tt.regexp)
			if tt.fail {
				require.ErrorContains(t, err, tt.failMsg)
			} else {
				require.NoError(t, err) // Give us predictably ordered results.
				SortScore(charts)

				l := len(tt.expect)
				require.Len(t, charts, len(tt.expect))
				// For empty result sets, just keep going.
				if l != 0 {
					for i, got := range charts {
						ex := tt.expect[i]
						assert.Equalf(t, got.Name, ex.Name, "[%d]: Expected name %q, got %q", i, ex.Name, got.Name)
					}
				}
			}
		})
	}
}

func TestSearchByNameAll(t *testing.T) {
	// Test with the All bit turned on.
	i := loadTestIndex(t, true)
	cs, err := i.Search("santa-maria", 100, false)
	require.NoError(t, err)
	assert.Lenf(t, cs, 2, "expected 2 charts, got %d", len(cs))
}

func TestCalcScore(t *testing.T) {
	i := NewIndex()

	fields := []string{"aaa", "bbb", "ccc", "ddd"}
	matchline := strings.Join(fields, sep)
	r := i.calcScore(2, matchline)
	assert.Equalf(t, 0, r, "Expected 0, got %d", r)
	r = i.calcScore(5, matchline)
	assert.Equalf(t, 1, r, "Expected 1, got %d", r)
	r = i.calcScore(10, matchline)
	assert.Equalf(t, 2, r, "Expected 2, got %d", r)
	r = i.calcScore(14, matchline)
	assert.Equalf(t, 3, r, "Expected 3, got %d", r)
}
