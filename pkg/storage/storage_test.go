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

func TestStorageCreate(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

	// create fake release
	rls := ReleaseTestData{
		Name:    "angry-beaver",
		Version: 1,
	}.ToRelease()

	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// fetch the release
	res, err := storage.Get(rls.Name, rls.Version)
	assertErrNil(t.Fatal, err, "QueryRelease")

	// verify the fetched and created release are the same
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestStorageUpdate(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

	// create fake release
	rls := ReleaseTestData{
		Name:    "angry-beaver",
		Version: 1,
		Status:  rspb.Status_DEPLOYED,
	}.ToRelease()

	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// modify the release
	rls.Info.Status.Code = rspb.Status_DELETED
	assertErrNil(t.Fatal, storage.Update(rls), "UpdateRelease")

	// retrieve the updated release
	res, err := storage.Get(rls.Name, rls.Version)
	assertErrNil(t.Fatal, err, "QueryRelease")

	// verify updated and fetched releases are the same.
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestStorageDelete(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

	// create fake release
	rls := ReleaseTestData{
		Name:    "angry-beaver",
		Version: 1,
	}.ToRelease()
	rls2 := ReleaseTestData{
		Name:    "angry-beaver",
		Version: 2,
	}.ToRelease()

	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")
	assertErrNil(t.Fatal, storage.Create(rls2), "StoreRelease")

	// delete the release
	res, err := storage.Delete(rls.Name, rls.Version)
	assertErrNil(t.Fatal, err, "DeleteRelease")

	// verify updated and fetched releases are the same.
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}

	hist, err := storage.History(rls.Name)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// We have now deleted one of the two records.
	if len(hist) != 1 {
		t.Errorf("expected 1 record for deleted release version, got %d", len(hist))
	}

	if hist[0].Version != 2 {
		t.Errorf("Expected version to be 2, got %d", hist[0].Version)
	}
}

func TestStorageList(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

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

func TestStorageDeployed(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"
	const vers = int32(4)

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: rspb.Status_DEPLOYED}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	rls, err := storage.Last(name)
	if err != nil {
		t.Fatalf("Failed to query for deployed release: %s\n", err)
	}

	switch {
	case rls == nil:
		t.Fatalf("Release is nil")
	case rls.Name != name:
		t.Fatalf("Expected release name %q, actual %q\n", name, rls.Name)
	case rls.Version != vers:
		t.Fatalf("Expected release version %d, actual %d\n", vers, rls.Version)
	case rls.Info.Status.Code != rspb.Status_DEPLOYED:
		t.Fatalf("Expected release status 'DEPLOYED', actual %s\n", rls.Info.Status.Code)
	}
}

func TestStorageHistory(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: rspb.Status_DEPLOYED}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	h, err := storage.History(name)
	if err != nil {
		t.Fatalf("Failed to query for release history (%q): %s\n", name, err)
	}
	if len(h) != 4 {
		t.Fatalf("Release history (%q) is empty\n", name)
	}
}

func TestStorageRemoveLeastRecent(t *testing.T) {
	storage := Init(driver.NewMemory())
	storage.Log = t.Logf

	// Make sure that specifying this at the outset doesn't cause any bugs.
	storage.MaxHistory = 10

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: rspb.Status_DEPLOYED}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}
	setup()

	// Because we have not set a limit, we expect 4.
	expect := 4
	if hist, err := storage.History(name); err != nil {
		t.Fatal(err)
	} else if len(hist) != expect {
		t.Fatalf("expected %d items in history, got %d", expect, len(hist))
	}

	storage.MaxHistory = 3
	rls5 := ReleaseTestData{Name: name, Version: 5, Status: rspb.Status_DEPLOYED}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls5), "Storing release 'angry-bird' (v5)")

	// On inserting the 5th record, we expect two records to be pruned from history.
	hist, err := storage.History(name)
	if err != nil {
		t.Fatal(err)
	} else if len(hist) != storage.MaxHistory {
		for _, item := range hist {
			t.Logf("%s %v", item.Name, item.Version)
		}
		t.Fatalf("expected %d items in history, got %d", storage.MaxHistory, len(hist))
	}

	// We expect the existing records to be 3, 4, and 5.
	for i, item := range hist {
		v := int(item.Version)
		if expect := i + 3; v != expect {
			t.Errorf("Expected release %d, got %d", expect, v)
		}
	}
}

func TestStorageLast(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"

	// Set up storage with test releases.
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: rspb.Status_SUPERSEDED}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: rspb.Status_FAILED}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	h, err := storage.Last(name)
	if err != nil {
		t.Fatalf("Failed to query for release history (%q): %s\n", name, err)
	}

	if h.Version != 4 {
		t.Errorf("Expected revision 4, got %d", h.Version)
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
