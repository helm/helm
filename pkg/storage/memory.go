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

package storage

import (
	"errors"
	"sync"

	"k8s.io/helm/pkg/proto/hapi/release"
)

// Memory is an in-memory ReleaseStorage implementation.
type Memory struct {
	sync.RWMutex
	releases map[string]*release.Release
}

// NewMemory creates a new in-memory storage.
func NewMemory() *Memory {
	return &Memory{
		releases: map[string]*release.Release{},
	}
}

// ErrNotFound indicates that a release is not found.
var ErrNotFound = errors.New("release not found")

// Read returns the named Release.
//
// If the release is not found, an ErrNotFound error is returned.
func (m *Memory) Read(k string) (*release.Release, error) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.releases[k]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}

// Create sets a release.
func (m *Memory) Create(rel *release.Release) error {
	m.Lock()
	defer m.Unlock()
	m.releases[rel.Name] = rel
	return nil
}

// Update sets a release.
func (m *Memory) Update(rel *release.Release) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.releases[rel.Name]; !ok {
		return ErrNotFound
	}

	// FIXME: When Release is done, we need to do this right by marking the old
	// release as superseded, and creating a new release.
	m.releases[rel.Name] = rel
	return nil
}

// Delete removes a release.
func (m *Memory) Delete(name string) (*release.Release, error) {
	m.Lock()
	defer m.Unlock()
	rel, ok := m.releases[name]
	if !ok {
		return nil, ErrNotFound
	}
	delete(m.releases, name)
	return rel, nil
}

// List returns all releases whose status is not Status_DELETED.
func (m *Memory) List() ([]*release.Release, error) {
	m.RLock()
	defer m.RUnlock()
	buf := []*release.Release{}
	for _, v := range m.releases {
		if v.Info.Status.Code != release.Status_DELETED {
			buf = append(buf, v)
		}
	}
	return buf, nil
}

// Query searches all releases for matches.
func (m *Memory) Query(labels map[string]string) ([]*release.Release, error) {
	m.RLock()
	defer m.RUnlock()
	return []*release.Release{}, errors.New("not implemented")
}

// History returns the history of this release, in the form of a series of releases.
func (m *Memory) History(name string) ([]*release.Release, error) {
	// TODO: This _should_ return all of the releases with the given name, sorted
	// by LastDeployed, regardless of status.
	r, err := m.Read(name)
	if err != nil {
		return []*release.Release{}, err
	}
	return []*release.Release{r}, nil
}
