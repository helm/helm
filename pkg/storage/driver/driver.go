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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"fmt"

	"github.com/pkg/errors"

	rspb "helm.sh/helm/v3/pkg/release"
)

var (
	// ErrReleaseNotFound indicates that a release is not found.
	ErrReleaseNotFound = errors.New("release: not found")
	// ErrReleaseExists indicates that a release already exists.
	ErrReleaseExists = errors.New("release: already exists")
	// ErrInvalidKey indicates that a release key could not be parsed.
	ErrInvalidKey = errors.New("release: invalid key")
	// ErrNoDeployedReleases indicates that there are no releases with the given key in the deployed state
	ErrNoDeployedReleases = errors.New("has no deployed releases")
)

// StorageDriverError records an error and the release name that caused it
type StorageDriverError struct {
	ReleaseName string
	Err         error
}

func (e *StorageDriverError) Error() string {
	return fmt.Sprintf("%q %s", e.ReleaseName, e.Err.Error())
}

func (e *StorageDriverError) Unwrap() error { return e.Err }

func NewErrNoDeployedReleases(releaseName string) error {
	return &StorageDriverError{
		ReleaseName: releaseName,
		Err:         ErrNoDeployedReleases,
	}
}

// Creator is the interface that wraps the Create method.
//
// Create stores the release or returns ErrReleaseExists
// if an identical release already exists.
type Creator interface {
	Create(key string, rls *rspb.Release) error
}

// Updator is the interface that wraps the Update method.
//
// Update updates an existing release or returns
// ErrReleaseNotFound if the release does not exist.
type Updator interface {
	Update(key string, rls *rspb.Release) error
}

// Deletor is the interface that wraps the Delete method.
//
// Delete deletes the release named by key or returns
// ErrReleaseNotFound if the release does not exist.
type Deletor interface {
	Delete(key string) (*rspb.Release, error)
}

// Queryor is the interface that wraps the Get and List methods.
//
// Get returns the release named by key or returns ErrReleaseNotFound
// if the release does not exist.
//
// List returns the set of all releases that satisfy the filter predicate.
//
// Query returns the set of all releases that match the provided label set.
type Queryor interface {
	Get(key string) (*rspb.Release, error)
	List(filter func(*rspb.Release) bool) ([]*rspb.Release, error)
	Query(labels map[string]string) ([]*rspb.Release, error)
}

// Driver is the interface composed of Creator, Updator, Deletor, and Queryor
// interfaces. It defines the behavior for storing, updating, deleted,
// and retrieving Helm releases from some underlying storage mechanism,
// e.g. memory, configmaps.
type Driver interface {
	Creator
	Updator
	Deletor
	Queryor
	Name() string
}
