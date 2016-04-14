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

// Get returns the named Release.
//
// If the release is not found, an ErrNotFound error is returned.
func (m *Memory) Get(k string) (*hapi.Release, error) {
	v, ok := m.releases[k]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}

// Set sets a release.
//
// TODO: Is there any reason why Set doesn't just use the release name?
func (m *Memory) Set(k string, rel *hapi.Release) error {
	m.releases[k] = rel
	return nil
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
