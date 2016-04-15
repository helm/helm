package storage

import (
	"errors"

	"github.com/deis/tiller/pkg/hapi"
)

// Memory is an in-memory ReleaseStorage implementation.
type Memory struct {
	releases map[string]*hapi.Release
}

func NewMemory() *Memory {
	return &Memory{
		releases: map[string]*hapi.Release{},
	}
}

var ErrNotFound = errors.New("release not found")

// Read returns the named Release.
//
// If the release is not found, an ErrNotFound error is returned.
func (m *Memory) Read(k string) (*hapi.Release, error) {
	v, ok := m.releases[k]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}

// Create sets a release.
func (m *Memory) Create(rel *hapi.Release) error {
	m.releases[rel.Name] = rel
	return nil
}

var ErrNoRelease = errors.New("no release found")

// Update sets a release.
func (m *Memory) Update(rel *hapi.Release) error {
	if _, ok := m.releases[rel.Name]; !ok {
		return ErrNoRelease
	}

	// FIXME: When Release is done, we need to do this right by marking the old
	// release as superseded, and creating a new release.
	m.releases[rel.Name] = rel
	return nil
}

func (m *Memory) Delete(name string) (*hapi.Release, error) {
	rel, ok := m.releases[name]
	if !ok {
		return nil, ErrNoRelease
	}
	delete(m.releases, name)
	return rel, nil
}

// List returns all releases
func (m *Memory) List() ([]*hapi.Release, error) {
	buf := make([]*hapi.Release, len(m.releases))
	i := 0
	for _, v := range m.releases {
		buf[i] = v
		i++
	}
	return buf, nil
}
func (m *Memory) Query(labels map[string]string) ([]*hapi.Release, error) {
	return []*hapi.Release{}, errors.New("Cannot implement until hapi.Release is defined.")
}
