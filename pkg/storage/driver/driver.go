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
	"errors"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

var (
	// ErrReleaseNotFound indicates that a release is not found.
	ErrReleaseNotFound = errors.New("release: not found")
	// ErrReleaseExists indicates that a release already exists.
	ErrReleaseExists = errors.New("release: already exists")
	// ErrDriverAction indicates the storage driver failed to execute the requested action.
	ErrDriverAction = errors.New("driver: failed to perform action")
)

// Creator stores a release.
type Creator interface {
	Create(rls *rspb.Release) error
}

// Updator updates an existing release or returns
// ErrReleaseNotFound if the release does not exist.
type Updator interface {
	Update(rls *rspb.Release) error
}

// Deletor deletes the release named by key or returns
// ErrReleaseNotFound if the release does not exist.
type Deletor interface {
	Delete(key string) (*rspb.Release, error)
}

// Queryor defines the behavior on accessing a release from storage.
type Queryor interface {
	// Get returns the release named by key or returns ErrReleaseNotFound
	// if the release does not exist.
	Get(key string) (*rspb.Release, error)
	// List returns the set of all releases that satisfy the filter predicate.
	List(filter func(*rspb.Release) bool) ([]*rspb.Release, error)
}

// Driver defines the behavior for storing, updating, deleted, and retrieving
// tiller releases from some underlying storage mechanism, e.g. memory, configmaps.
type Driver interface {
	Creator
	Updator
	Deletor
	Queryor
}
