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

package storage

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	require.NoError(t, storage.Create(rls), "StoreRelease")

	// fetch the release
	res, err := storage.Get(rls.Name, rls.Version)
	require.NoError(t, err, "QueryRelease")

	// verify the fetched and created release are the same
	require.Equalf(t, rls, res, "Expected %v, got %v", rls, res)
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

	require.NoError(t, storage.Create(rls), "StoreRelease")

	// modify the release
	rls.Info.Status = common.StatusUninstalled
	require.NoError(t, storage.Update(rls), "UpdateRelease")

	// retrieve the updated release
	res, err := storage.Get(rls.Name, rls.Version)
	require.NoError(t, err, "QueryRelease")

	// verify updated and fetched releases are the same.
	require.Equalf(t, rls, res, "Expected %v, got %v", rls, res)
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

	require.NoError(t, storage.Create(rls), "StoreRelease")
	require.NoError(t, storage.Create(rls2), "StoreRelease")

	// delete the release
	res, err := storage.Delete(rls.Name, rls.Version)
	require.NoError(t, err, "DeleteRelease")

	// verify updated and fetched releases are the same.
	require.Equalf(t, rls, res, "Expected %v, got %v", rls, res)

	hist, err := storage.History(rls.Name)
	require.NoError(t, err)

	rhist, err := releaseListToV1List(hist)
	require.NoError(t, err)

	// We have now deleted one of the two records.
	assert.Len(t, rhist, 1, "expected 1 record for deleted release version, got %d", len(hist))

	assert.Equal(t, 2, rhist[0].Version, "Expected version to be 2, got %d", rhist[0].Version)
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
		require.NoError(t, storage.Create(rls0), "Storing release 'rls0'")
		require.NoError(t, storage.Create(rls1), "Storing release 'rls1'")
		require.NoError(t, storage.Create(rls2), "Storing release 'rls2'")
		require.NoError(t, storage.Create(rls3), "Storing release 'rls3'")
		require.NoError(t, storage.Create(rls4), "Storing release 'rls4'")
		require.NoError(t, storage.Create(rls5), "Storing release 'rls5'")
		require.NoError(t, storage.Create(rls6), "Storing release 'rls6'")
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
		require.NoError(t, err, tt.Description)
		// verify the count of releases returned
		assert.Len(t, list, tt.NumExpected, "ListReleases(%s): expected %d, actual %d", tt.Description, tt.NumExpected, len(list))
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	rls, err := storage.Last(name)
	require.NoError(t, err, "Failed to query for deployed release")

	rel, err := releaserToV1Release(rls)
	require.NoError(t, err)

	require.NotNil(t, rls, "Release is nil")
	require.Equal(t, name, rel.Name, "Expected release name %q, actual %q\n", name, rel.Name)
	require.Equal(t, vers, rel.Version, "Expected release version %d, actual %d\n", vers, rel.Version)
	require.Equal(t, common.StatusDeployed, rel.Info.Status, "Expected release status 'DEPLOYED', actual %s\n", rel.Info.Status.String())
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	rls, err := storage.Deployed(name)
	require.NoError(t, err, "Failed to query for deployed release")

	rel, err := releaserToV1Release(rls)
	require.NoError(t, err)

	require.NotNil(t, rls, "Release is nil")
	require.Equal(t, name, rel.Name, "Expected release name %q, actual %q\n", name, rel.Name)
	require.Equal(t, vers, rel.Version, "Expected release version %d, actual %d\n", vers, rel.Version)
	require.Equal(t, common.StatusDeployed, rel.Info.Status, "Expected release status 'DEPLOYED', actual %s\n", rel.Info.Status.String())
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	h, err := storage.History(name)
	require.NoError(t, err, "Failed to query for release history (%q)", name)
	require.Len(t, h, 4, "Release history (%q) is empty\n", name)
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
	storage := Init(NewMaxHistoryMockDriver(driver.NewMemory()))

	storage.MaxHistory = 1

	const name = "angry-bird"

	// setup storage with test releases
	setup := func() {
		// release records
		rls1 := ReleaseTestData{Name: name, Version: 1, Status: common.StatusSuperseded}.ToRelease()

		// create the release records in the storage
		require.NoError(t, storage.Driver.Create(makeKey(rls1.Name, rls1.Version), rls1), "Storing release 'angry-bird' (v1)")
	}
	setup()

	rls2 := ReleaseTestData{Name: name, Version: 2, Status: common.StatusSuperseded}.ToRelease()
	wantErr := errMaxHistoryMockDriverSomethingHappened
	gotErr := storage.Create(rls2)
	require.ErrorIs(t, gotErr, wantErr, "Storing release 'angry-bird' (v2) should return the error %#v, but returned %#v", wantErr, gotErr)
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}
	setup()

	// Because we have not set a limit, we expect 4.
	expect := 4
	hist, err := storage.History(name)
	require.NoError(t, err)
	require.Equal(t, len(hist), expect, "expected %d items in history, got %d", expect, len(hist))

	storage.MaxHistory = 3
	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusDeployed}.ToRelease()
	require.NoError(t, storage.Create(rls5), "Storing release 'angry-bird' (v5)")

	// On inserting the 5th record, we expect two records to be pruned from history.
	hist, err = storage.History(name)
	require.NoError(t, err)
	rhist, err := releaseListToV1List(hist)
	require.NoError(t, err)
	if !assert.Len(t, rhist, storage.MaxHistory) {
		for _, item := range rhist {
			t.Logf("%s %v", item.Name, item.Version)
		}
	}
	require.Len(t, rhist, storage.MaxHistory)

	// We expect the existing records to be 3, 4, and 5.
	for i, item := range rhist {
		v := item.Version
		expect := i + 3
		assert.Equalf(t, v, expect, "Expected release %d, got %d", expect, v)
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}
	setup()

	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusFailed}.ToRelease()
	require.NoError(t, storage.Create(rls5), "Storing release 'angry-bird' (v5)")

	// On inserting the 5th record, we expect a total of 3 releases, but we expect version 2
	// (the only deployed release), to still exist
	hist, err := storage.History(name)
	require.NoError(t, err)
	if !assert.Len(t, hist, storage.MaxHistory) {
		rhist, err := releaseListToV1List(hist)
		require.NoError(t, err)
		for _, item := range rhist {
			t.Logf("%s %v", item.Name, item.Version)
		}
	}
	require.Len(t, hist, storage.MaxHistory)

	expectedVersions := map[int]bool{
		2: true,
		4: true,
		5: true,
	}

	rhist, err := releaseListToV1List(hist)
	require.NoError(t, err)
	for _, item := range rhist {
		assert.Truef(t, expectedVersions[item.Version], "Release version %d, found when not expected", item.Version)
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")
	}

	setup()

	h, err := storage.Last(name)
	require.NoError(t, err, "Failed to query for release history (%q)", name)

	rel, err := releaserToV1Release(h)
	require.NoError(t, err)

	assert.Equal(t, 4, rel.Version, "Expected revision 4, got %d", rel.Version)
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
		require.NoError(t, storage.Create(rls0), "Storing release 'angry-bird' (v1)")
		require.NoError(t, storage.Create(rls1), "Storing release 'angry-bird' (v2)")
		require.NoError(t, storage.Create(rls2), "Storing release 'angry-bird' (v3)")
		require.NoError(t, storage.Create(rls3), "Storing release 'angry-bird' (v4)")

		hist, err := storage.History(name)
		require.NoError(t, err)

		wantHistoryLen := 4
		require.Len(t, hist, wantHistoryLen, "expected history of release %q to contain %d releases, got %d", name, wantHistoryLen, len(hist))
	}

	setup()

	rls5 := ReleaseTestData{Name: name, Version: 5, Status: common.StatusFailed}.ToRelease()
	require.NoError(t, storage.Create(rls5), "Failed to create a new release version")

	hist, err := storage.History(name)
	require.NoError(t, err)

	rhist, err := releaseListToV1List(hist)
	require.NoError(t, err)
	for i, rel := range rhist {
		wantVersion := i + 2
		require.Equal(t, wantVersion, rel.Version, "Expected history release %d version to equal %d, got %d", i+1, wantVersion, rel.Version)

		wantStatus := common.StatusFailed
		require.Equal(t, wantStatus, rel.Info.Status, "Expected history release %d status to equal %q, got %q", i+1, wantStatus, rel.Info.Status)
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

func TestStorageGetsLoggerFromDriver(t *testing.T) {
	d := driver.NewMemory()
	l := &mockSLogHandler{}
	d.SetLogger(l)
	s := Init(d)
	_, _ = s.Get("doesnt-matter", 123)
	require.True(t, l.Called, "Expected storage to use driver's logger, but it did not")
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
