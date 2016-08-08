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

package storage // import "k8s.io/helm/pkg/storage"

import (
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/storage/driver"
	"log"
)

// Storage represents a storage engine for a Release.
type Storage struct {
	driver.Driver
}

// Get retrieves the release from storage. An error is returned
// if the storage driver failed to fetch the release, or the
// release identified by key does not exist.
func (s *Storage) Get(key string) (*rspb.Release, error) {
	log.Printf("Getting release %q from storage\n", key)
	return s.Driver.Get(key)
}

// Create creates a new storage entry holding the release. An
// error is returned if the storage driver failed to store the
// release, or a release with identical an key already exists.
func (s *Storage) Create(rls *rspb.Release) error {
	log.Printf("Create release %q in storage\n", rls.Name)
	return s.Driver.Create(rls)
}

// Update update the release in storage. An error is returned if the
// storage backend fails to update the release or if the release
// does not exist.
func (s *Storage) Update(rls *rspb.Release) error {
	log.Printf("Updating %q in storage\n", rls.Name)
	return s.Driver.Update(rls)
}

// Delete deletes the release from storage. An error is returned if
// the storage backend fails to delete the release or if the release
// does not exist.
func (s *Storage) Delete(key string) (*rspb.Release, error) {
	log.Printf("Deleting release %q from storage\n", key)
	return s.Driver.Delete(key)
}

// ListReleases returns all releases from storage. An error is returned if the
// storage backend fails to retrieve the releases.
func (s *Storage) ListReleases() ([]*rspb.Release, error) {
	log.Println("Listing all releases in storage")
	return s.Driver.List(func(_ *rspb.Release) bool { return true })
}

// ListDeleted returns all releases with Status == DELETED. An error is returned
// if the storage backend fails to retrieve the releases.
func (s *Storage) ListDeleted() ([]*rspb.Release, error) {
	log.Println("List deleted releases in storage")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return StatusFilter(rspb.Status_DELETED).Check(rls)
	})
}

// ListDeployed returns all releases with Status == DEPLOYED. An error is returned
// if the storage backend fails to retrieve the releases.
func (s *Storage) ListDeployed() ([]*rspb.Release, error) {
	log.Println("Listing all deployed releases in storage")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return StatusFilter(rspb.Status_DEPLOYED).Check(rls)
	})
}

// ListFilterAll returns the set of releases satisfying satisfying the predicate
// (filter0 && filter1 && ... && filterN), i.e. a Release is included in the results
// if and only if all filters return true.
func (s *Storage) ListFilterAll(filters ...FilterFunc) ([]*rspb.Release, error) {
	log.Println("Listing all releases with filter")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return All(filters...).Check(rls)
	})
}

// ListFilterAny returns the set of releases satisfying satisfying the predicate
// (filter0 || filter1 || ... || filterN), i.e. a Release is included in the results
// if at least one of the filters returns true.
func (s *Storage) ListFilterAny(filters ...FilterFunc) ([]*rspb.Release, error) {
	log.Println("Listing any releases with filter")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return Any(filters...).Check(rls)
	})
}

// Init initializes a new storage backend with the driver d.
// If d is nil, the default in-memory driver is used.
func Init(d driver.Driver) *Storage {
	if d == nil {
		d = driver.NewMemory()
	}
	return &Storage{Driver: d}
}
