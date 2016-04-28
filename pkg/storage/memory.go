package storage

import (
	"errors"
	"sync"

	"github.com/deis/tiller/pkg/proto/hapi/release"
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

// List returns all releases.
func (m *Memory) List() ([]*release.Release, error) {
	m.RLock()
	defer m.RUnlock()
	buf := make([]*release.Release, len(m.releases))
	i := 0
	for _, v := range m.releases {
		buf[i] = v
		i++
	}
	return buf, nil
}

// Query searches all releases for matches.
func (m *Memory) Query(labels map[string]string) ([]*release.Release, error) {
	m.RLock()
	defer m.RUnlock()
	return []*release.Release{}, errors.New("Cannot implement until release.Release is defined.")
}
