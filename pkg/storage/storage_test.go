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

package storage // import "helm.sh/helm/v4/pkg/storage"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
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
		t.Fatalf("Expected %v, got %v", rls, res)
	}
}

func TestStorageUpdate(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

	// create fake release
	rls := ReleaseTestData{
		Name:    "angry-beaver",
		Version: 1,
		Status:  common.StatusDeployed,
	}.ToRelease()

	assertErrNil(t.Fatal, storage.Create(rls), "StoreRelease")

	// modify the release
	rls.Info.Status = common.StatusUninstalled
	assertErrNil(t.Fatal, storage.Update(rls), "UpdateRelease")

	// retrieve the updated release
	res, err := storage.Get(rls.Name, rls.Version)
	assertErrNil(t.Fatal, err, "QueryRelease")

	// verify updated and fetched releases are the same.
	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %v, got %v", rls, res)
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
		t.Fatalf("Expected %v, got %v", rls, res)
	}

	hist, err := storage.History(rls.Name)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	rhist, err := releaseListToV1List(hist)
	assert.NoError(t, err)

	// We have now deleted one of the two records.
	if len(rhist) != 1 {
		t.Errorf("expected 1 record for deleted release version, got %d", len(hist))
	}

	if rhist[0].Version != 2 {
		t.Errorf("Expected version to be 2, got %d", rhist[0].Version)
	}
}

func TestStorageList(t *testing.T) {
	// initialize storage
	storage := Init(driver.NewMemory())

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: "happy-catdog", Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: "livid-human", Status: common.StatusSuperseded}.ToRelease()
		rls2 := ReleaseTestData{Name: "relaxed-cat", Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: "hungry-hippo", Status: common.StatusDeployed}.ToRelease()
		rls4 := ReleaseTestData{Name: "angry-beaver", Status: common.StatusDeployed}.ToRelease()
		rls5 := ReleaseTestData{Name: "opulent-frog", Status: common.StatusUninstalled}.ToRelease()
		rls6 := ReleaseTestData{Name: "happy-liger", Status: common.StatusUninstalled}.ToRelease()

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
		ListFunc    func() ([]release.Releaser, error)
	}{
		{"ListDeployed", 2, storage.ListDeployed},
		{"ListReleases", 7, storage.ListReleases},
		{"ListUninstalled", 2, storage.ListUninstalled},
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
	const vers = 4

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusDeployed}.ToRelease()

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

	rel, err := releaserToV1Release(rls)
	assert.NoError(t, err)

	switch {
	case rls == nil:
		t.Fatalf("Release is nil")
	case rel.Name != name:
		t.Fatalf("Expected release name %q, actual %q\n", name, rel.Name)
	case rel.Version != vers:
		t.Fatalf("Expected release version %d, actual %d\n", vers, rel.Version)
	case rel.Info.Status != common.StatusDeployed:
		t.Fatalf("Expected release status 'DEPLOYED', actual %s\n", rel.Info.Status.String())
	}
}

func TestStorageDeployedWithCorruption(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"
	const vers = int(4)

	// setup storage with test releases
	setup := func() {
		// release records (notice odd order and corruption)
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusDeployed}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusDeployed}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	rls, err := storage.Deployed(name)
	if err != nil {
		t.Fatalf("Failed to query for deployed release: %s\n", err)
	}

	rel, err := releaserToV1Release(rls)
	assert.NoError(t, err)

	switch {
	case rls == nil:
		t.Fatalf("Release is nil")
	case rel.Name != name:
		t.Fatalf("Expected release name %q, actual %q\n", name, rel.Name)
	case rel.Version != vers:
		t.Fatalf("Expected release version %d, actual %d\n", vers, rel.Version)
	case rel.Info.Status != common.StatusDeployed:
		t.Fatalf("Expected release status 'DEPLOYED', actual %s\n", rel.Info.Status.String())
	}
}

func TestStorageHistory(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusDeployed}.ToRelease()

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

var errMaxHistoryMockDriverSomethingHappened = errors.New("something happened")

type MaxHistoryMockDriver struct {
	Driver driver.Driver
}

func NewMaxHistoryMockDriver(d driver.Driver) *MaxHistoryMockDriver {
	return &MaxHistoryMockDriver{Driver: d}
}
func (d *MaxHistoryMockDriver) Create(key string, rls release.Releaser) error {
	return d.Driver.Create(key, rls)
}
func (d *MaxHistoryMockDriver) Update(key string, rls release.Releaser) error {
	return d.Driver.Update(key, rls)
}
func (d *MaxHistoryMockDriver) Delete(_ string) (release.Releaser, error) {
	return nil, errMaxHistoryMockDriverSomethingHappened
}
func (d *MaxHistoryMockDriver) Get(key string) (release.Releaser, error) {
	return d.Driver.Get(key)
}
func (d *MaxHistoryMockDriver) List(filter func(release.Releaser) bool) ([]release.Releaser, error) {
	return d.Driver.List(filter)
}
func (d *MaxHistoryMockDriver) Query(labels map[string]string) ([]release.Releaser, error) {
	return d.Driver.Query(labels)
}
func (d *MaxHistoryMockDriver) Name() string {
	return d.Driver.Name()
}

func TestMaxHistoryErrorHandling(t *testing.T) {
	//func TestStorageRemoveLeastRecentWithError(t *testing.T) {
	storage := Init(NewMaxHistoryMockDriver(driver.NewMemory()))

	storage.MaxHistory = 1

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls1 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Driver.Create(makeKey(rls1.Name, rls1.Version), rls1), "Storing release 'angry-bird' (v1)")
	}
	setup()

	rls2 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
	wantErr := errMaxHistoryMockDriverSomethingHappened
	gotErr := storage.Create(rls2)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("Storing release 'angry-bird' (v2) should return the error %#v, but returned %#v", wantErr, gotErr)
	}
}

func TestStorageRemoveLeastRecent(t *testing.T) {
	storage := Init(driver.NewMemory())

	// Make sure that specifying this at the outset doesn't cause any bugs.
	storage.MaxHistory = 10

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusDeployed}.ToRelease()

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
	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusDeployed}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls5), "Storing release 'angry-bird' (v5)")

	// On inserting the 5th record, we expect two records to be pruned from history.
	hist, err := storage.History(name)
	assert.NoError(t, err)
	rhist, err := releaseListToV1List(hist)
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	} else if len(rhist) != storage.MaxHistory {
		for _, item := range rhist {
			t.Logf("%s %v", item.Name, item.Version)
		}
		t.Fatalf("expected %d items in history, got %d", storage.MaxHistory, len(rhist))
	}

	// We expect the existing records to be 3, 4, and 5.
	for i, item := range rhist {
		v := item.Version
		if expect := i + 3; v != expect {
			t.Errorf("Expected release %d, got %d", expect, v)
		}
	}
}

func TestStorageDoNotDeleteDeployed(t *testing.T) {
	storage := Init(driver.NewMemory())
	storage.MaxHistory = 3

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusDeployed}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusFailed}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusFailed}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}
	setup()

	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusFailed}.ToRelease()
	assertErrNil(t.Fatal, storage.Create(rls5), "Storing release 'angry-bird' (v5)")

	// On inserting the 5th record, we expect a total of 3 releases, but we expect version 2
	// (the only deployed release), to still exist
	hist, err := storage.History(name)
	if err != nil {
		t.Fatal(err)
	} else if len(hist) != storage.MaxHistory {
		rhist, err := releaseListToV1List(hist)
		assert.NoError(t, err)
		for _, item := range rhist {
			t.Logf("%s %v", item.Name, item.Version)
		}
		t.Fatalf("expected %d items in history, got %d", storage.MaxHistory, len(rhist))
	}

	expectedVersions := map[int]bool{
		2: true,
		4: true,
		5: true,
	}

	rhist, err := releaseListToV1List(hist)
	assert.NoError(t, err)
	for _, item := range rhist {
		if !expectedVersions[item.Version] {
			t.Errorf("Release version %d, found when not expected", item.Version)
		}
	}
}

func TestStorageLast(t *testing.T) {
	storage := Init(driver.NewMemory())

	const name = "angry-bird"

	// Set up storage with test releases.
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusSuperseded}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusFailed}.ToRelease()

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

	rel, err := releaserToV1Release(h)
	assert.NoError(t, err)

	if rel.Version != 4 {
		t.Errorf("Expected revision 4, got %d", rel.Version)
	}
}

// TestUpgradeInitiallyFailedReleaseWithHistoryLimit tests a case when there are no deployed release yet, but history limit has been
// reached: the has-no-deployed-releases error should not occur in such case.
func TestUpgradeInitiallyFailedReleaseWithHistoryLimit(t *testing.T) {
	storage := Init(driver.NewMemory())
	storage.MaxHistory = 4

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls0 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusFailed}.ToRelease()
		rls1 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusFailed}.ToRelease()
		rls2 := ReleaseTestData{Name: name, Version: 3, Status: common.StatusFailed}.ToRelease()
		rls3 := ReleaseTestData{Name: name, Version: 4, Status: common.StatusFailed}.ToRelease()

		// create the release records in the storage
		assertErrNil(t.Fatal, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		assertErrNil(t.Fatal, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		assertErrNil(t.Fatal, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		assertErrNil(t.Fatal, storage.Create(rls3), "Storing release 'angry-bird' (v4)")

		hist, err := storage.History(name)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		wantHistoryLen := 4
		if len(hist) != wantHistoryLen {
			t.Fatalf("expected history of release %q to contain %d releases, got %d", name, wantHistoryLen, len(hist))
		}
	}

	setup()

	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusFailed}.ToRelease()
	err := storage.Create(rls5)
	if err != nil {
		t.Fatalf("Failed to create a new release version: %s", err)
	}

	hist, err := storage.History(name)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	rhist, err := releaseListToV1List(hist)
	assert.NoError(t, err)
	for i, rel := range rhist {
		wantVersion := i + 2
		if rel.Version != wantVersion {
			t.Fatalf("Expected history release %d version to equal %d, got %d", i+1, wantVersion, rel.Version)
		}

		wantStatus := common.StatusFailed
		if rel.Info.Status != wantStatus {
			t.Fatalf("Expected history release %d status to equal %q, got %q", i+1, wantStatus, rel.Info.Status)
		}
	}
}

type ReleaseTestData struct {
	Name      string
	Version   int
	Manifest  string
	Namespace string
	Status    common.Status
}

func (test ReleaseTestData) ToRelease() *rspb.Release {
	return &rspb.Release{
		Name:      test.Name,
		Version:   test.Version,
		Manifest:  test.Manifest,
		Namespace: test.Namespace,
		Info:      &rspb.Info{Status: test.Status},
	}
}

func assertErrNil(eh func(args ...interface{}), err error, message string) {
	if err != nil {
		eh(fmt.Sprintf("%s: %q", message, err))
	}
}

func TestStorageGetsLoggerFromDriver(t *testing.T) {
	d := driver.NewMemory()
	l := &mockSLogHandler{}
	d.SetLogger(l)
	s := Init(d)
	_, _ = s.Get("doesnt-matter", 123)
	if !l.Called {
		t.Fatalf("Expected storage to use driver's logger, but it did not")
	}
}

type mockSLogHandler struct {
	Called bool
}

func (m *mockSLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (m *mockSLogHandler) Handle(context.Context, slog.Record) error {
	m.Called = true
	return nil
}

func (m *mockSLogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return m
}

func (m *mockSLogHandler) WithGroup(string) slog.Handler {
	return m
}
