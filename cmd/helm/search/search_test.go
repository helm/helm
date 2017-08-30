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

package search

import (
	"strings"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/repo"
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
	for i := 0; i < len(expectScore); i++ {
		if expectScore[i] != in[i].Score {
			t.Errorf("Sort error on index %d: expected %d, got %d", i, expectScore[i], in[i].Score)
		}
	}
	// Test Name
	for i := 0; i < len(expect); i++ {
		if expect[i] != in[i].Name {
			t.Errorf("Sort error: expected %s, got %s", expect[i], in[i].Name)
		}
	}

	// Test version of last two items
	if in[5].Chart.Version != "1.2.4" {
		t.Errorf("Expected 1.2.4, got %s", in[5].Chart.Version)
	}
	if in[6].Chart.Version != "1.2.3" {
		t.Error("Expected 1.2.3 to be last")
	}
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
			URLs: []string{"http://example.com/charts/santa-maria-1.2.2.tgz"},
			Metadata: &chart.Metadata{
				Name:        "santa-maria",
				Version:     "1.2.2",
				Description: "Three boat",
			},
		},
	},
}

func loadTestIndex(t *testing.T, all bool) *Index {
	i := NewIndex()
	i.AddRepo("testing", &repo.IndexFile{Entries: indexfileEntries}, all)
	i.AddRepo("ztesting", &repo.IndexFile{Entries: map[string]repo.ChartVersions{
		"pinta": {
			{
				URLs: []string{"http://example.com/charts/pinta-2.0.0.tgz"},
				Metadata: &chart.Metadata{
					Name:        "pinta",
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
	if len(all) != 4 {
		t.Errorf("Expected 4 entries, got %d", len(all))
	}

	i = loadTestIndex(t, true)
	all = i.All()
	if len(all) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(all))
	}
}

func TestAddRepo_Sort(t *testing.T) {
	i := loadTestIndex(t, true)
	sr, err := i.Search("TESTING/SANTA-MARIA", 100, false)
	if err != nil {
		t.Fatal(err)
	}
	SortScore(sr)

	ch := sr[0]
	expect := "1.2.3"
	if ch.Chart.Version != expect {
		t.Errorf("Expected %q, got %q", expect, ch.Chart.Version)
	}
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
				{Name: "ztesting/pinta"},
			},
		},
		{
			name:  "repo-specific search for one result",
			query: "ztesting/pinta",
			expect: []*Result{
				{Name: "ztesting/pinta"},
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
				{Name: "ztesting/pinta"},
			},
		},
		{
			name:  "description upper search, two results",
			query: "TWO",
			expect: []*Result{
				{Name: "testing/pinta"},
				{Name: "ztesting/pinta"},
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

		charts, err := i.Search(tt.query, 100, tt.regexp)
		if err != nil {
			if tt.fail {
				if !strings.Contains(err.Error(), tt.failMsg) {
					t.Fatalf("%s: Unexpected error message: %s", tt.name, err)
				}
				continue
			}
			t.Fatalf("%s: %s", tt.name, err)
		}
		// Give us predictably ordered results.
		SortScore(charts)

		l := len(charts)
		if l != len(tt.expect) {
			t.Fatalf("%s: Expected %d result, got %d", tt.name, len(tt.expect), l)
		}
		// For empty result sets, just keep going.
		if l == 0 {
			continue
		}

		for i, got := range charts {
			ex := tt.expect[i]
			if got.Name != ex.Name {
				t.Errorf("%s[%d]: Expected name %q, got %q", tt.name, i, ex.Name, got.Name)
			}
		}

	}
}

func TestSearchByNameAll(t *testing.T) {
	// Test with the All bit turned on.
	i := loadTestIndex(t, true)
	cs, err := i.Search("santa-maria", 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Errorf("expected 2 charts, got %d", len(cs))
	}
}

func TestCalcScore(t *testing.T) {
	i := NewIndex()

	fields := []string{"aaa", "bbb", "ccc", "ddd"}
	matchline := strings.Join(fields, sep)
	if r := i.calcScore(2, matchline); r != 0 {
		t.Errorf("Expected 0, got %d", r)
	}
	if r := i.calcScore(5, matchline); r != 1 {
		t.Errorf("Expected 1, got %d", r)
	}
	if r := i.calcScore(10, matchline); r != 2 {
		t.Errorf("Expected 2, got %d", r)
	}
	if r := i.calcScore(14, matchline); r != 3 {
		t.Errorf("Expected 3, got %d", r)
	}
}
