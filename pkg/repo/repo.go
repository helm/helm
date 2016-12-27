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
	"time"

	"github.com/ghodss/yaml"
)

// ErrRepoOutOfDate indicates that the repository file is out of date, but
// is fixable.
var ErrRepoOutOfDate = errors.New("repository file is out of date")

// RepositoryFile represents the repositories.yaml file in $HELM_HOME
type RepositoryFile struct {
	APIVersion   string                   `json:"apiVersion"`
	Generated    time.Time                `json:"generated"`
	Repositories []*ChartRepositoryConfig `json:"repositories"`
}

// NewRepositoryFile generates an empty repositories file.
//
// Generated and APIVersion are automatically set.
func NewRepositoryFile() *RepositoryFile {
	return &RepositoryFile{
		APIVersion:   APIVersionV1,
		Generated:    time.Now(),
		Repositories: []*ChartRepositoryConfig{},
	}
}

// LoadRepositoryFile takes a file at the given path and returns a RepositoryFile object
//
// If this returns ErrRepoOutOfDate, it also returns a recovered RepositoryFile that
// can be saved as a replacement to the out of date file.
func LoadRepositoryFile(path string) (*RepositoryFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	r := &RepositoryFile{}
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
		r := NewRepositoryFile()
		for k, v := range m {
			r.Add(&ChartRepositoryConfig{
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
func (r *RepositoryFile) Add(re ...*ChartRepositoryConfig) {
	r.Repositories = append(r.Repositories, re...)
}

// Update attempts to replace one or more repo entries in a repo file. If an
// entry with the same name doesn't exist in the repo file it will add it.
func (r *RepositoryFile) Update(re ...*ChartRepositoryConfig) {
	for _, target := range re {
		found := false
		for j, repo := range r.Repositories {
			if repo.Name == target.Name {
				r.Repositories[j] = target
				found = true
				break
			}
		}
		if !found {
			r.Add(target)
		}
	}
}

// Has returns true if the given name is already a repository name.
func (r *RepositoryFile) Has(name string) bool {
	for _, rf := range r.Repositories {
		if rf.Name == name {
			return true
		}
	}
	return false
}

// Remove removes the entry from the list of repositories.
func (r *RepositoryFile) Remove(name string) bool {
	cp := []*ChartRepositoryConfig{}
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
func (r *RepositoryFile) WriteFile(path string, perm os.FileMode) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, perm)
}
