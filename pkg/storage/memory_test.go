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

package storage

import (
	"testing"

	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestCreate(t *testing.T) {
	k := "test-1"
	r := &release.Release{Name: k}

	ms := NewMemory()
	if err := ms.Create(r); err != nil {
		t.Fatalf("Failed create: %s", err)
	}

	if ms.releases[k].Name != k {
		t.Errorf("Unexpected release name: %s", ms.releases[k].Name)
	}
}

func TestRead(t *testing.T) {
	k := "test-1"
	r := &release.Release{Name: k}

	ms := NewMemory()
	ms.Create(r)

	if out, err := ms.Read(k); err != nil {
		t.Errorf("Could not get %s: %s", k, err)
	} else if out.Name != k {
		t.Errorf("Expected %s, got %s", k, out.Name)
	}
}

func TestHistory(t *testing.T) {
	k := "test-1"
	r := &release.Release{Name: k}

	ms := NewMemory()
	ms.Create(r)

	if out, err := ms.History(k); err != nil {
		t.Errorf("Could not get %s: %s", k, err)
	} else if len(out) != 1 {
		t.Fatalf("Expected 1 release, got %d", len(out))
	} else if out[0].Name != k {
		t.Errorf("Expected %s, got %s", k, out[0].Name)
	}
}

func TestUpdate(t *testing.T) {
	k := "test-1"
	r := &release.Release{Name: k}

	ms := NewMemory()
	if err := ms.Create(r); err != nil {
		t.Fatalf("Failed create: %s", err)
	}
	if err := ms.Update(r); err != nil {
		t.Fatalf("Failed update: %s", err)
	}

	if ms.releases[k].Name != k {
		t.Errorf("Unexpected release name: %s", ms.releases[k].Name)
	}
}

func TestList(t *testing.T) {
	ms := NewMemory()
	rels := []string{"a", "b", "c"}

	for _, k := range rels {
		ms.Create(&release.Release{
			Name: k,
			Info: &release.Info{
				Status: &release.Status{Code: release.Status_UNKNOWN},
			},
		})
		ms.Create(&release.Release{
			Name: "deleted-should-not-show-up",
			Info: &release.Info{
				Status: &release.Status{Code: release.Status_DELETED},
			},
		})
	}

	l, err := ms.List()
	if err != nil {
		t.Error(err)
	}

	if len(l) != 3 {
		t.Errorf("Expected 3, got %d", len(l))
	}

	for _, n := range rels {
		foundN := false
		for _, rr := range l {
			if rr.Name == n {
				foundN = true
				break
			}
		}
		if !foundN {
			t.Errorf("Did not find %s in list.", n)
		}
	}
}

func TestQuery(t *testing.T) {
	t.Skip("Not Implemented")
}
