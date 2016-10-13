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

package repo // import "k8s.io/helm/pkg/repo"

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/provenance"
)

// ErrRepoOutOfDate indicates that the repository file is out of date, but
// is fixable.
var ErrRepoOutOfDate = errors.New("repository file is out of date")

// ChartRepository represents a chart repository
type ChartRepository struct {
	RootPath   string
	URL        string // URL of repository
	ChartPaths []string
	IndexFile  *IndexFile
}

// Entry represents one repo entry in a repositories listing.
type Entry struct {
	Name  string `json:"name"`
	Cache string `json:"cache"`
	URL   string `json:"url"`
}

// RepoFile represents the repositories.yaml file in $HELM_HOME
type RepoFile struct {
	APIVersion   string    `json:"apiVersion"`
	Generated    time.Time `json:"generated"`
	Repositories []*Entry  `json:"repositories"`
}

// NewRepoFile generates an empty repositories file.
//
// Generated and APIVersion are automatically set.
func NewRepoFile() *RepoFile {
	return &RepoFile{
		APIVersion:   APIVersionV1,
		Generated:    time.Now(),
		Repositories: []*Entry{},
	}
}

// LoadRepositoriesFile takes a file at the given path and returns a RepoFile object
//
// If this returns ErrRepoOutOfDate, it also returns a recovered RepoFile that
// can be saved as a replacement to the out of date file.
func LoadRepositoriesFile(path string) (*RepoFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	r := &RepoFile{}
	err = yaml.Unmarshal(b, r)
	if err != nil {
		return nil, err
	}

	// File is either corrupt, or is from before v2.0.0-Alpha.5
	if r.APIVersion == "" {
		m := map[string]string{}
		if err = yaml.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		r := NewRepoFile()
		for k, v := range m {
			r.Add(&Entry{
				Name:  k,
				URL:   v,
				Cache: fmt.Sprintf("%s-index.yaml", k),
			})
		}
		return r, ErrRepoOutOfDate
	}

	return r, nil
}

// Add adds one or more repo entries to a repo file.
func (r *RepoFile) Add(re ...*Entry) {
	r.Repositories = append(r.Repositories, re...)
}

// Has returns true if the given name is already a repository name.
func (r *RepoFile) Has(name string) bool {
	for _, rf := range r.Repositories {
		if rf.Name == name {
			return true
		}
	}
	return false
}

// Remove removes the entry from the list of repositories.
func (r *RepoFile) Remove(name string) bool {
	cp := []*Entry{}
	found := false
	for _, rf := range r.Repositories {
		if rf.Name == name {
			found = true
			continue
		}
		cp = append(cp, rf)
	}
	r.Repositories = cp
	return found
}

// WriteFile writes a repositories file to the given path.
func (r *RepoFile) WriteFile(path string, perm os.FileMode) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, perm)
}

// LoadChartRepository loads a directory of charts as if it were a repository.
//
// It requires the presence of an index.yaml file in the directory.
//
// This function evaluates the contents of the directory and
// returns a ChartRepository
func LoadChartRepository(dir, url string) (*ChartRepository, error) {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	r := &ChartRepository{RootPath: dir, URL: url}

	// FIXME: Why are we recursively walking directories?
	// FIXME: Why are we not reading the repositories.yaml to figure out
	// what repos to use?
	filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if strings.Contains(f.Name(), "-index.yaml") {
				i, err := LoadIndexFile(path)
				if err != nil {
					return nil
				}
				r.IndexFile = i
			} else if strings.HasSuffix(f.Name(), ".tgz") {
				r.ChartPaths = append(r.ChartPaths, path)
			}
		}
		return nil
	})
	return r, nil
}

func (r *ChartRepository) saveIndexFile() error {
	index, err := yaml.Marshal(r.IndexFile)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(r.RootPath, indexPath), index, 0644)
}

// Index generates an index for the chart repository and writes an index.yaml file.
func (r *ChartRepository) Index() error {
	if r.IndexFile == nil {
		r.IndexFile = NewIndexFile()
	}

	for _, path := range r.ChartPaths {
		ch, err := chartutil.Load(path)
		if err != nil {
			return err
		}

		digest, err := provenance.DigestFile(path)
		if err != nil {
			return err
		}

		if !r.IndexFile.Has(ch.Metadata.Name, ch.Metadata.Version) {
			r.IndexFile.Add(ch.Metadata, path, r.URL, digest)
		}
		// TODO: If a chart exists, but has a different Digest, should we error?
	}
	r.IndexFile.SortEntries()
	return r.saveIndexFile()
}
