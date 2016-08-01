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
package driver // import "k8s.io/helm/pkg/storage/driver"

import (
	"sync"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

// Memory is an in-memory storage driver implementation.
type Memory struct {
	sync.RWMutex
	cache map[string]*rspb.Release
}

func NewMemory() *Memory {
	return &Memory{cache: map[string]*rspb.Release{}}
}

// Get returns the release named by key.
func (mem *Memory) Get(key string) (*rspb.Release, error) {
	defer unlock(mem.rlock())

	if rls, ok := mem.cache[key]; ok {
		return rls, nil
	}
	return nil, ErrReleaseNotFound
}

// List returns all releases whose status is not Status_DELETED.
func (mem *Memory) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	defer unlock(mem.rlock())
	
	var releases []*rspb.Release
	for k := range mem.cache {
		if filter(mem.cache[k]) {
			releases = append(releases, mem.cache[k])
		}
	}
	return releases, nil
}

// Create creates a new release or error.
func (mem *Memory) Create(rls *rspb.Release) error {
	defer unlock(mem.wlock())
	mem.cache[rls.Name] = rls
	return nil
}

// Update updates a release or error.
func (mem *Memory) Update(rls *rspb.Release) error {
	defer unlock(mem.wlock())

	if old, ok := mem.cache[rls.Name]; ok {
		// FIXME: when release update is complete, old release should
		// be marked as superseded, creating the new release.
		_ = old

		mem.cache[rls.Name] = rls
		return nil
	}
	return ErrReleaseNotFound
}

// Delete deletes a release or error.
func (mem *Memory) Delete(key string) (*rspb.Release, error) {
	defer unlock(mem.wlock())

	if old, ok := mem.cache[key]; ok {
		old.Info.Status.Code = rspb.Status_DELETED
		delete(mem.cache, key)
		return old, nil
	}
	return nil, ErrReleaseNotFound
}

func (mem *Memory) wlock() func() {
	mem.Lock()
	return func() {
		mem.Unlock()
	}
}

func (mem *Memory) rlock() func() {
	mem.RLock()
	return func() {
		mem.RUnlock()
	}
}

func unlock(fn func()) { fn() }
