/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package repo

import (
	"fmt"
	"strings"
)

type inmemRepoService struct {
	repositories map[string]Repo
}

// NewInmemRepoService returns a new memory based repository service.
func NewInmemRepoService() RepoService {
	rs := &inmemRepoService{
		repositories: make(map[string]Repo),
	}

	r, err := NewPublicGCSRepo(nil)
	if err == nil {
		rs.Create(r)
	}

	return rs
}

// List returns the list of all known chart repositories
func (rs *inmemRepoService) List() ([]Repo, error) {
	ret := []Repo{}
	for _, r := range rs.repositories {
		ret = append(ret, r)
	}

	return ret, nil
}

// Create adds a known repository to the list
func (rs *inmemRepoService) Create(repository Repo) error {
	rs.repositories[repository.GetName()] = repository
	return nil
}

// Get returns the repository with the given name
func (rs *inmemRepoService) Get(name string) (Repo, error) {
	r, ok := rs.repositories[name]
	if !ok {
		return nil, fmt.Errorf("Failed to find repository named %s", name)
	}

	return r, nil
}

// GetByURL returns the repository that backs the given URL
func (rs *inmemRepoService) GetByURL(URL string) (Repo, error) {
	var found Repo
	for _, r := range rs.repositories {
		rURL := r.GetURL()
		if strings.HasPrefix(URL, rURL) {
			if found == nil || len(found.GetURL()) < len(rURL) {
				found = r
			}
		}
	}

	if found == nil {
		return nil, fmt.Errorf("Failed to find repository for url: %s", URL)
	}

	return found, nil
}

// Delete removes a known repository from the list
func (rs *inmemRepoService) Delete(name string) error {
	_, ok := rs.repositories[name]
	if !ok {
		return fmt.Errorf("Failed to find repository named %s", name)
	}

	delete(rs.repositories, name)
	return nil
}
