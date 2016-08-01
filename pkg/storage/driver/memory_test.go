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
	"testing"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestMemoryGet(t *testing.T) {
	key := "test-1"
	rls := &rspb.Release{Name: key}

	mem := NewMemory()
	mem.Create(rls)

	res, err := mem.Get(key)
	switch {
	case err != nil:
		t.Errorf("Could not get %s: %s", key, err)
	case res.Name != key:
		t.Errorf("Expected %s, got %s", key, res.Name)
	}
}

func TestMemoryAll(t *testing.T) {
	t.Skip("MemoryAll")
}

func TestMemoryCreate(t *testing.T) {
	key := "test-1"
	rls := &rspb.Release{Name: key}

	mem := NewMemory()
	err := mem.Create(rls)

	switch {
	case err != nil:
		t.Fatalf("Failed create: %s", err)
	case mem.cache[key].Name != key:
		t.Errorf("Unexpected release name: %s", mem.cache[key].Name)
	}
}

func TestMemoryUpdate(t *testing.T) {
	key := "test-1"
	rls := &rspb.Release{Name: key}

	mem := NewMemory()
	if err := mem.Create(rls); err != nil {
		t.Fatalf("Failed create: %s", err)
	}
	if err := mem.Update(rls); err != nil {
		t.Fatalf("Failed update: %s", err)
	}
	if mem.cache[key].Name != key {
		t.Errorf("Unexpected release name: %s", mem.cache[key].Name)
	}
}

func TestMemoryDelete(t *testing.T) {
	key := "test-1"
	rls := &rspb.Release{Name: key}

	mem := NewMemory()
	if err := mem.Create(rls); err != nil {
		t.Fatalf("Failed create: %s", err)
	}
	
	res, err := mem.Delete(key)
	switch {
	case err != nil:
		t.Fatalf("Failed delete: %s", err)
	case mem.cache[key] != nil:
		t.Errorf("Expected nil, got %s", mem.cache[key])
	case res.Info.Status.Code != release.Status_DELETED:
		t.Errorf("Expected Status_DELETED, got %s", res.Info.Status.Code)
	}
}