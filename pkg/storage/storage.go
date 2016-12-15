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
	"fmt"
	"log"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	relutil "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/storage/driver"
)

// Storage represents a storage engine for a Release.
type Storage struct {
	driver.Driver
}

// Get retrieves the release from storage. An error is returned
// if the storage driver failed to fetch the release, or the
// release identified by the key, version pair does not exist.
func (s *Storage) Get(name string, version int32) (*rspb.Release, error) {
	log.Printf("Getting release %q (v%d) from storage\n", name, version)
	return s.Driver.Get(makeKey(name, version))
}

// Create creates a new storage entry holding the release. An
// error is returned if the storage driver failed to store the
// release, or a release with identical an key already exists.
func (s *Storage) Create(rls *rspb.Release) error {
	log.Printf("Create release %q (v%d) in storage\n", rls.Name, rls.Version)
	return s.Driver.Create(makeKey(rls.Name, rls.Version), rls)
}

// Update update the release in storage. An error is returned if the
// storage backend fails to update the release or if the release
// does not exist.
func (s *Storage) Update(rls *rspb.Release) error {
	log.Printf("Updating %q (v%d) in storage\n", rls.Name, rls.Version)
	return s.Driver.Update(makeKey(rls.Name, rls.Version), rls)
}

// Delete deletes the release from storage. An error is returned if
// the storage backend fails to delete the release or if the release
// does not exist.
func (s *Storage) Delete(name string, version int32) (*rspb.Release, error) {
	log.Printf("Deleting release %q (v%d) from storage\n", name, version)
	return s.Driver.Delete(makeKey(name, version))
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
		return relutil.StatusFilter(rspb.Status_DELETED).Check(rls)
	})
}

// ListDeployed returns all releases with Status == DEPLOYED. An error is returned
// if the storage backend fails to retrieve the releases.
func (s *Storage) ListDeployed() ([]*rspb.Release, error) {
	log.Println("Listing all deployed releases in storage")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return relutil.StatusFilter(rspb.Status_DEPLOYED).Check(rls)
	})
}

// ListFilterAll returns the set of releases satisfying satisfying the predicate
// (filter0 && filter1 && ... && filterN), i.e. a Release is included in the results
// if and only if all filters return true.
func (s *Storage) ListFilterAll(fns ...relutil.FilterFunc) ([]*rspb.Release, error) {
	log.Println("Listing all releases with filter")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return relutil.All(fns...).Check(rls)
	})
}

// ListFilterAny returns the set of releases satisfying satisfying the predicate
// (filter0 || filter1 || ... || filterN), i.e. a Release is included in the results
// if at least one of the filters returns true.
func (s *Storage) ListFilterAny(fns ...relutil.FilterFunc) ([]*rspb.Release, error) {
	log.Println("Listing any releases with filter")
	return s.Driver.List(func(rls *rspb.Release) bool {
		return relutil.Any(fns...).Check(rls)
	})
}

// Deployed returns the deployed release with the provided release name, or
// returns ErrReleaseNotFound if not found.
func (s *Storage) Deployed(name string) (*rspb.Release, error) {
	log.Printf("Getting deployed release from '%s' history\n", name)

	ls, err := s.Driver.Query(map[string]string{
		"NAME":   name,
		"OWNER":  "TILLER",
		"STATUS": "DEPLOYED",
	})
	switch {
	case err != nil:
		return nil, err
	case len(ls) == 0:
		return nil, fmt.Errorf("'%s' has no deployed releases", name)
	default:
		return ls[0], nil
	}
}

// History returns the revision history for the release with the provided name, or
// returns ErrReleaseNotFound if no such release name exists.
func (s *Storage) History(name string) ([]*rspb.Release, error) {
	log.Printf("Getting release history for '%s'\n", name)

	l, err := s.Driver.Query(map[string]string{"NAME": name, "OWNER": "TILLER"})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Last fetches the last revision of the named release.
func (s *Storage) Last(name string) (*rspb.Release, error) {
	h, err := s.History(name)
	if err != nil {
		return nil, err
	}
	if len(h) == 0 {
		return nil, fmt.Errorf("no revision for release %q", name)
	}

	relutil.Reverse(h, relutil.SortByRevision)
	return h[0], nil
}

// makeKey concatenates a release name and version into
// a string with format ```<release_name>#v<version>```.
// This key is used to uniquely identify storage objects.
func makeKey(rlsname string, version int32) string {
	return fmt.Sprintf("%s.v%d", rlsname, version)
}

// Init initializes a new storage backend with the driver d.
// If d is nil, the default in-memory driver is used.
func Init(d driver.Driver) *Storage {
	// default driver is in memory
	if d == nil {
		d = driver.NewMemory()
	}
	return &Storage{Driver: d}
}
