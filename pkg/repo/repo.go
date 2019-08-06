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

package repo // import "helm.sh/helm/pkg/repo"

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// ErrRepoOutOfDate indicates that the repository file is out of date, but
// is fixable.
var ErrRepoOutOfDate = errors.New("repository file is out of date")

// File represents the repositories.yaml file
type File struct {
	APIVersion   string    `json:"apiVersion"`
	Generated    time.Time `json:"generated"`
	Repositories []*Entry  `json:"repositories"`
}

// NewFile generates an empty repositories file.
//
// Generated and APIVersion are automatically set.
func NewFile() *File {
	return &File{
		APIVersion:   APIVersionV1,
		Generated:    time.Now(),
		Repositories: []*Entry{},
	}
}

// LoadFile takes a file at the given path and returns a File object
//
// If this returns ErrRepoOutOfDate, it also returns a recovered File that
// can be saved as a replacement to the out of date file.
func LoadFile(path string) (*File, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Errorf("couldn't load repositories file (%s).\nYou might need to run `helm init`", path)
		}
		return nil, err
	}

	r := &File{}
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
		r := NewFile()
		for k, v := range m {
			r.Add(&Entry{
				Name: k,
				URL:  v,
			})
		}
		return r, ErrRepoOutOfDate
	}

	return r, nil
}

// Add adds one or more repo entries to a repo file.
func (r *File) Add(re ...*Entry) {
	r.Repositories = append(r.Repositories, re...)
}

// Update attempts to replace one or more repo entries in a repo file. If an
// entry with the same name doesn't exist in the repo file it will add it.
func (r *File) Update(re ...*Entry) {
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
func (r *File) Has(name string) bool {
	for _, rf := range r.Repositories {
		if rf.Name == name {
			return true
		}
	}
	return false
}

// Remove removes the entry from the list of repositories.
func (r *File) Remove(name string) bool {
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
func (r *File) WriteFile(path string, perm os.FileMode) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, perm)
}
