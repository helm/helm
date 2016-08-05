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
	"reflect"
	"testing"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/storage/driver"
)

var storage = Init(driver.NewMemory())

func TestStorageCreate(t *testing.T) {
	// create fake release
	rls := ReleaseTestData{Name: "angry-beaver"}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// fetch the release
	res, err := storage.Get(rls.Name)
	assertErrNil(t.Fatal, err, "QueryRelease")

	// verify the fetched and created release are the same
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestStorageUpdate(t *testing.T) {
	// create fake release
	rls := ReleaseTestData{Name: "angry-beaver"}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// modify the release
	rls.Version = 2
	rls.Manifest = "new-manifest"
	assertErrNil(t.Fatal, storage.Update(rls), "UpdateRelease")

	// retrieve the updated release
	res, err := storage.Get(rls.Name)
	assertErrNil(t.Fatal, err, "QueryRelease")

	// verify updated and fetched releases are the same.
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestStorageDelete(t *testing.T) {
	// create fake release
	rls := ReleaseTestData{Name: "angry-beaver"}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// delete the release
	res, err := storage.Delete(rls.Name)
	assertErrNil(t.Fatal, err, "DeleteRelease")

	// verify updated and fetched releases are the same.
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestStorageList(t *testing.T) {
	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: "happy-catdog", Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls1 := ReleaseTestData{Name: "livid-human", Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls2 := ReleaseTestData{Name: "relaxed-cat", Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls3 := ReleaseTestData{Name: "hungry-hippo", Status: rspb.Status_DEPLOYED}.ToRelease()
		rls4 := ReleaseTestData{Name: "angry-beaver", Status: rspb.Status_DEPLOYED}.ToRelease()
		rls5 := ReleaseTestData{Name: "opulent-frog", Status: rspb.Status_DELETED}.ToRelease()
		rls6 := ReleaseTestData{Name: "happy-liger", Status: rspb.Status_DELETED}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'rls0'")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'rls1'")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'rls2'")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'rls3'")
		assertErrNil(t.Fatal, storage.Create(rls4), "Storing release 'rls4'")
		assertErrNil(t.Fatal, storage.Create(rls5), "Storing release 'rls5'")
		assertErrNil(t.Fatal, storage.Create(rls6), "Storing release 'rls6'")
	}

	var listTests = []struct {
		Description string
		NumExpected int
		ListFunc    func() ([]*rspb.Release, error)
	}{
		{"ListDeleted", 2, storage.ListDeleted},
		{"ListDeployed", 2, storage.ListDeployed},
		{"ListReleases", 7, storage.ListReleases},
	}

	setup()

	for _, tt := range listTests {
		list, err := tt.ListFunc()
		assertErrNil(t.Fatal, err, tt.Description)
		// verify the count of releases returned
		if len(list) != tt.NumExpected {
			t.Errorf("ListReleases(%s): expected %d, actual %d",
				tt.Description,
				tt.NumExpected,
				len(list))
		}
	}
}

type ReleaseTestData struct {
	Name      string
	Version   int32
	Manifest  string
	Namespace string
	Status    rspb.Status_Code
}

func (test ReleaseTestData) ToRelease() *rspb.Release {
	return &rspb.Release{
		Name:      test.Name,
		Version:   test.Version,
		Manifest:  test.Manifest,
		Namespace: test.Namespace,
		Info:      &rspb.Info{Status: &rspb.Status{Code: test.Status}},
	}
}

func assertErrNil(eh func(args ...interface{}), err error, message string) {
	if err != nil {
		eh(fmt.Sprintf("%s: %q", message, err))
	}
}
