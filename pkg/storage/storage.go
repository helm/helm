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
)

type Storage struct {
	drv driver.Driver
}

func (st *Storage) StoreRelease(rls *rspb.Release) error {
	return st.drv.Create(rls)
}

func (st *Storage) UpdateRelease(rls *rspb.Release) error {
	return st.drv.Update(rls)
}

func (st *Storage) QueryRelease(key string) (*rspb.Release, error) {
	return st.drv.Get(key)
}

func (st *Storage) DeleteRelease(key string) (*rspb.Release, error) {
	return st.drv.Delete(key)
}

func Init(drv driver.Driver) *Storage {
	if drv == nil {
		drv = driver.NewMemory()
	}
	return &Storage{drv: drv}
}
