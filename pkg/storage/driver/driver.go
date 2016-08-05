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
	ErrReleaseNotFound = errors.New("release not found")
	// Temporary error while WIP.
	ErrNotImplemented = errors.New("not implemented")
)

type Creator interface {
	Create(*rspb.Release) error
}

type Updator interface {
	Update(*rspb.Release) error
}

type Deletor interface {
	Delete(string) (*rspb.Release, error)
}

type Queryor interface {
	Get(string) (*rspb.Release, error)

	All(string, ...interface{}) ([]*rspb.Release, error)
}

type Driver interface {
	Creator
	Updator
	Deletor
	Queryor
}
