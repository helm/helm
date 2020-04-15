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

package driver

import (
	"strconv"
	"strings"
	"sync"

	rspb "helm.sh/helm/v3/pkg/release"
)

var _ Driver = (*Memory)(nil)

const (
	// MemoryDriverName is the string name of this driver.
	MemoryDriverName = "Memory"

	defaultNamespace = "default"
)

// A map of release names to list of release records
type memReleases map[string]records

// Memory is the in-memory storage driver implementation.
type Memory struct {
	sync.RWMutex
	namespace string
	// A map of namespaces to releases
	cache map[string]memReleases
}

// NewMemory initializes a new memory driver.
func NewMemory() *Memory {
	return &Memory{cache: map[string]memReleases{}, namespace: "default"}
}

// SetNamespace sets a specific namespace in which releases will be accessed.
// An empty string indicates all namespaces (for the list operation)
func (mem *Memory) SetNamespace(ns string) {
	mem.namespace = ns
}

// Name returns the name of the driver.
func (mem *Memory) Name() string {
	return MemoryDriverName
}

// Get returns the release named by key or returns ErrReleaseNotFound.
func (mem *Memory) Get(key string) (*rspb.Release, error) {
	defer unlock(mem.rlock())

	keyWithoutPrefix := strings.TrimPrefix(key, "sh.helm.release.v1.")
	switch elems := strings.Split(keyWithoutPrefix, ".v"); len(elems) {
	case 2:
		name, ver := elems[0], elems[1]
		if _, err := strconv.Atoi(ver); err != nil {
			return nil, ErrInvalidKey
		}
		if recs, ok := mem.cache[mem.namespace][name]; ok {
			if r := recs.Get(key); r != nil {
				return r.rls, nil
			}
		}
		return nil, ErrReleaseNotFound
	default:
		return nil, ErrInvalidKey
	}
}

// List returns the list of all releases such that filter(release) == true
func (mem *Memory) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	defer unlock(mem.rlock())

	var ls []*rspb.Release
	for namespace := range mem.cache {
		if mem.namespace != "" {
			// Should only list releases of this namespace
			namespace = mem.namespace
		}
		for _, recs := range mem.cache[namespace] {
			recs.Iter(func(_ int, rec *record) bool {
				if filter(rec.rls) {
					ls = append(ls, rec.rls)
				}
				return true
			})
		}
		if mem.namespace != "" {
			// Should only list releases of this namespace
			break
		}
	}
	return ls, nil
}

// Query returns the set of releases that match the provided set of labels
func (mem *Memory) Query(keyvals map[string]string) ([]*rspb.Release, error) {
	defer unlock(mem.rlock())

	var lbs labels

	lbs.init()
	lbs.fromMap(keyvals)

	var ls []*rspb.Release
	for namespace := range mem.cache {
		if mem.namespace != "" {
			// Should only query releases of this namespace
			namespace = mem.namespace
		}
		for _, recs := range mem.cache[namespace] {
			recs.Iter(func(_ int, rec *record) bool {
				// A query for a release name that doesn't exist (has been deleted)
				// can cause rec to be nil.
				if rec == nil {
					return false
				}
				if rec.lbs.match(lbs) {
					ls = append(ls, rec.rls)
				}
				return true
			})
		}
		if mem.namespace != "" {
			// Should only query releases of this namespace
			break
		}
	}

	if len(ls) == 0 {
		return nil, ErrReleaseNotFound
	}

	return ls, nil
}

// Create creates a new release or returns ErrReleaseExists.
func (mem *Memory) Create(key string, rls *rspb.Release) error {
	defer unlock(mem.wlock())

	// For backwards compatibility, we protect against an unset namespace
	namespace := rls.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	mem.SetNamespace(namespace)

	if _, ok := mem.cache[namespace]; !ok {
		mem.cache[namespace] = memReleases{}
	}

	if recs, ok := mem.cache[namespace][rls.Name]; ok {
		if err := recs.Add(newRecord(key, rls)); err != nil {
			return err
		}
		mem.cache[namespace][rls.Name] = recs
		return nil
	}
	mem.cache[namespace][rls.Name] = records{newRecord(key, rls)}
	return nil
}

// Update updates a release or returns ErrReleaseNotFound.
func (mem *Memory) Update(key string, rls *rspb.Release) error {
	defer unlock(mem.wlock())

	// For backwards compatibility, we protect against an unset namespace
	namespace := rls.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	mem.SetNamespace(namespace)

	if _, ok := mem.cache[namespace]; ok {
		if rs, ok := mem.cache[namespace][rls.Name]; ok && rs.Exists(key) {
			rs.Replace(key, newRecord(key, rls))
			return nil
		}
	}
	return ErrReleaseNotFound
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (mem *Memory) Delete(key string) (*rspb.Release, error) {
	defer unlock(mem.wlock())

	keyWithoutPrefix := strings.TrimPrefix(key, "sh.helm.release.v1.")
	elems := strings.Split(keyWithoutPrefix, ".v")

	if len(elems) != 2 {
		return nil, ErrInvalidKey
	}

	name, ver := elems[0], elems[1]
	if _, err := strconv.Atoi(ver); err != nil {
		return nil, ErrInvalidKey
	}
	if _, ok := mem.cache[mem.namespace]; ok {
		if recs, ok := mem.cache[mem.namespace][name]; ok {
			if r := recs.Remove(key); r != nil {
				// recs.Remove changes the slice reference, so we have to re-assign it.
				mem.cache[mem.namespace][name] = recs
				return r.rls, nil
			}
		}
	}
	return nil, ErrReleaseNotFound
}

// wlock locks mem for writing
func (mem *Memory) wlock() func() {
	mem.Lock()
	return func() { mem.Unlock() }
}

// rlock locks mem for reading
func (mem *Memory) rlock() func() {
	mem.RLock()
	return func() { mem.RUnlock() }
}

// unlock calls fn which reverses a mem.rlock or mem.wlock. e.g:
// ```defer unlock(mem.rlock())```, locks mem for reading at the
// call point of defer and unlocks upon exiting the block.
func unlock(fn func()) { fn() }
