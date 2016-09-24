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

/*Package search provides client-side repository searching.

This supports building an in-memory search index based on the contents of
multiple repositories, and then using string matching or regular expressions
to find matches.
*/
package search

import (
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"k8s.io/helm/pkg/repo"
)

// Result is a search result.
//
// Score indicates how close it is to match. The higher the score, the longer
// the distance.
type Result struct {
	Name  string
	Score int
}

// Index is a searchable index of chart information.
type Index struct {
	lines  map[string]string
	charts map[string]*repo.ChartRef
}

const sep = "\v"

// NewIndex creats a new Index.
func NewIndex() *Index {
	return &Index{lines: map[string]string{}, charts: map[string]*repo.ChartRef{}}
}

// AddRepo adds a repository index to the search index.
func (i *Index) AddRepo(rname string, ind *repo.IndexFile) {
	for name, ref := range ind.Entries {
		fname := filepath.Join(rname, name)
		i.lines[fname] = indstr(rname, ref)
		i.charts[fname] = ref
	}
}

// Entries returns the entries in an index.
func (i *Index) Entries() map[string]*repo.ChartRef {
	return i.charts
}

// Search searches an index for the given term.
//
// Threshold indicates the maximum score a term may have before being marked
// irrelevant. (Low score means higher relevance. Golf, not bowling.)
//
// If regexp is true, the term is treated as a regular expression. Otherwise,
// term is treated as a literal string.
func (i *Index) Search(term string, threshold int, regexp bool) ([]*Result, error) {
	if regexp == true {
		return i.SearchRegexp(term, threshold)
	}
	return i.SearchLiteral(term, threshold), nil
}

// calcScore calculates a score for a match.
func (i *Index) calcScore(index int, matchline string) int {

	// This is currently tied to the fact that sep is a single char.
	splits := []int{}
	s := rune(sep[0])
	for i, ch := range matchline {
		if ch == s {
			splits = append(splits, i)
		}
	}

	for i, pos := range splits {
		if index > pos {
			continue
		}
		return i
	}
	return len(splits)
}

// SearchLiteral does a literal string search (no regexp).
func (i *Index) SearchLiteral(term string, threshold int) []*Result {
	term = strings.ToLower(term)
	buf := []*Result{}
	for k, v := range i.lines {
		res := strings.Index(v, term)
		if score := i.calcScore(res, v); res != -1 && score < threshold {
			buf = append(buf, &Result{Name: k, Score: score})
		}
	}
	return buf
}

// SearchRegexp searches using a regular expression.
func (i *Index) SearchRegexp(re string, threshold int) ([]*Result, error) {
	matcher, err := regexp.Compile(re)
	if err != nil {
		return []*Result{}, err
	}
	buf := []*Result{}
	for k, v := range i.lines {
		ind := matcher.FindStringIndex(v)
		if len(ind) == 0 {
			continue
		}
		if score := i.calcScore(ind[0], v); ind[0] >= 0 && score < threshold {
			buf = append(buf, &Result{Name: k, Score: score})
		}
	}
	return buf, nil
}

// Chart returns the ChartRef for a particular name.
func (i *Index) Chart(name string) (*repo.ChartRef, error) {
	c, ok := i.charts[name]
	if !ok {
		return nil, errors.New("no such chart")
	}
	return c, nil
}

// SortScore does an in-place sort of the results.
//
// Lowest scores are highest on the list. Matching scores are subsorted alphabetically.
func SortScore(r []*Result) {
	sort.Sort(scoreSorter(r))
}

// scoreSorter sorts results by score, and subsorts by alpha Name.
type scoreSorter []*Result

// Len returns the length of this scoreSorter.
func (s scoreSorter) Len() int { return len(s) }

// Swap performs an in-place swap.
func (s scoreSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less compares a to b, and returns true if a is less than b.
func (s scoreSorter) Less(a, b int) bool {
	first := s[a]
	second := s[b]

	if first.Score > second.Score {
		return false
	}
	if first.Score < second.Score {
		return true
	}
	return first.Name < second.Name
}

func indstr(name string, ref *repo.ChartRef) string {
	i := ref.Name + sep + name + "/" + ref.Name + sep
	if ref.Chartfile != nil {
		i += ref.Chartfile.Description + sep + strings.Join(ref.Chartfile.Keywords, sep)
	}
	return strings.ToLower(i)
}
