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
	"sync"
)

type inmemRepoService struct {
	sync.RWMutex
	repositories map[string]IRepo
}

// NewInmemRepoService returns a new memory based repository service.
func NewInmemRepoService() IRepoService {
	rs := &inmemRepoService{
		repositories: make(map[string]IRepo),
	}

	r, err := NewPublicGCSRepo(nil)
	if err == nil {
		rs.CreateRepo(r)
	}

	return rs
}

// ListRepos returns the list of all known chart repositories
func (rs *inmemRepoService) ListRepos() (map[string]string, error) {
	rs.RLock()
	defer rs.RUnlock()

	ret := make(map[string]string)
	for _, r := range rs.repositories {
		ret[r.GetName()] = r.GetURL()
	}

	return ret, nil
}

// CreateRepo adds a known repository to the list
func (rs *inmemRepoService) CreateRepo(repository IRepo) error {
	rs.Lock()
	defer rs.Unlock()

	URL := repository.GetURL()
	name := repository.GetName()

	for u, r := range rs.repositories {
		if u == URL {
			return fmt.Errorf("Repository with URL %s already exists", URL)
		} else if r.GetName() == name {
			return fmt.Errorf("Repository with Name %s already exists", name)
		}
	}

	rs.repositories[URL] = repository
	return nil
}

// GetRepoByURL returns the repository with the given URL
func (rs *inmemRepoService) GetRepoByURL(URL string) (IRepo, error) {
	rs.RLock()
	defer rs.RUnlock()

	r, ok := rs.repositories[URL]
	if !ok {
		return nil, fmt.Errorf("No repository with URL %s", URL)
	}

	return r, nil
}

// GetRepoByChartURL returns the repository that backs the given chart URL
func (rs *inmemRepoService) GetRepoByChartURL(URL string) (IRepo, error) {
	rs.RLock()
	defer rs.RUnlock()

	cSplit := strings.Split(URL, "/")
	var found IRepo
	for _, r := range rs.repositories {
		rURL := r.GetURL()
		rSplit := strings.Split(rURL, "/")
		if hasPrefix(cSplit, rSplit) {
			if found == nil || len(found.GetURL()) < len(rURL) {
				found = r
			}
		}
	}

	if found == nil {
		return nil, fmt.Errorf("No repository for url %s", URL)
	}

	return found, nil
}

func hasPrefix(cSplit, rSplit []string) bool {
	if len(rSplit) > len(cSplit) {
		return false
	}

	for i := range rSplit {
		if rSplit[i] != cSplit[i] {
			return false
		}
	}

	return true
}

// DeleteRepo removes a known repository from the list
func (rs *inmemRepoService) DeleteRepo(URL string) error {
	rs.Lock()
	defer rs.Unlock()

	_, ok := rs.repositories[URL]
	if !ok {
		return fmt.Errorf("No repository with URL %s", URL)
	}

	delete(rs.repositories, URL)
	return nil
}

func (rs *inmemRepoService) GetRepoURLByName(name string) (string, error) {
	for url, r := range rs.repositories {
		if r.GetName() == name {
			return url, nil
		}
	}
	err := fmt.Errorf("No repository url found with name %s", name)
	return "", err
}
