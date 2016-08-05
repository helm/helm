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
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

type ConfigMaps struct {
	*Memory // simple cache
}

func NewConfigMaps() *ConfigMaps {
	return &ConfigMaps{Memory: NewMemory()}
}

// Get retrieves the releases named by key from the ConfigMap
func (cfg *ConfigMaps) Get(key string) (*rspb.Release, error) {
	return nil, ErrNotImplemented
}

// All returns all releases whose status is not Status_DELETED.
func (cfg *ConfigMaps) All(key string, opts ...interface{}) ([]*rspb.Release, error) {
	return nil, ErrNotImplemented
}

// Create creates a new release or error.
func (cfg *ConfigMaps) Create(rls *rspb.Release) error {
	return ErrNotImplemented
}

// Update updates a release or error.
func (cfg *ConfigMaps) Update(rls *rspb.Release) error {
	return ErrNotImplemented
}

// Delete deletes a release or error.
func (cfg *ConfigMaps) Delete(key string) (*rspb.Release, error) {
	return nil, ErrNotImplemented
}
