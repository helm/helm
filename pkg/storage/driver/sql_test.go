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

package driver

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestSQLName(t *testing.T) {
	sqlDriver, _ := newTestFixtureSQL(t)
	if sqlDriver.Name() != SQLDriverName {
		t.Errorf("Expected name to be %q, got %q", SQLDriverName, sqlDriver.Name())
	}
}

func TestSQLGet(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	body, err := encodeRelease(rel)
	if err != nil {
		t.Fatal(err)
	}

	sqlDriver, mock := newTestFixtureSQL(t)
	mock.
		ExpectQuery("SELECT body FROM releases WHERE key = ?").
		WithArgs(key).
		WillReturnRows(
			mock.NewRows([]string{
				"body",
			}).AddRow(
				body,
			),
		).RowsWillBeClosed()

	got, err := sqlDriver.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %v", err)
	}

	if !shallowReleaseEqual(rel, got) {
		t.Errorf("Expected release {%q}, got {%q}", rel, got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSQLList(t *testing.T) {
	body1, _ := encodeRelease(releaseStub("key-1", 1, "default", rspb.Status_DELETED))
	body2, _ := encodeRelease(releaseStub("key-2", 1, "default", rspb.Status_DELETED))
	body3, _ := encodeRelease(releaseStub("key-3", 1, "default", rspb.Status_DEPLOYED))
	body4, _ := encodeRelease(releaseStub("key-4", 1, "default", rspb.Status_DEPLOYED))
	body5, _ := encodeRelease(releaseStub("key-5", 1, "default", rspb.Status_SUPERSEDED))
	body6, _ := encodeRelease(releaseStub("key-6", 1, "default", rspb.Status_SUPERSEDED))

	sqlDriver, mock := newTestFixtureSQL(t)

	for i := 0; i < 3; i++ {
		mock.
			ExpectQuery("SELECT body FROM releases WHERE owner = 'TILLER'").
			WillReturnRows(
				mock.NewRows([]string{
					"body",
				}).
					AddRow(body1).
					AddRow(body2).
					AddRow(body3).
					AddRow(body4).
					AddRow(body5).
					AddRow(body6),
			).RowsWillBeClosed()
	}

	// list all deleted releases
	del, err := sqlDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DELETED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %v", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	// list all deployed releases
	dpl, err := sqlDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DEPLOYED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %v", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d:\n%v\n", len(dpl), dpl)
	}

	// list all superseded releases
	ssd, err := sqlDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_SUPERSEDED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %v", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d:\n%v\n", len(ssd), ssd)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlCreate(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	mock.ExpectBegin()
	mock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO releases (key, body, name, version, status, owner, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)")).
		WithArgs(key, body, rel.Name, int(rel.Version), rspb.Status_Code_name[int32(rel.Info.Status.Code)], "TILLER", int(time.Now().Unix())).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := sqlDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %q: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlCreateAlreadyExists(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	// Insert fails (primary key already exists)
	mock.ExpectBegin()
	mock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO releases (key, body, name, version, status, owner, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)")).
		WithArgs(key, body, rel.Name, int(rel.Version), rspb.Status_Code_name[int32(rel.Info.Status.Code)], "TILLER", int(time.Now().Unix())).
		WillReturnError(fmt.Errorf("dialect dependent SQL error"))

	// Let's check that we do make sure the error is due to a release already existing
	mock.
		ExpectQuery(regexp.QuoteMeta("SELECT key FROM releases WHERE key = ?")).
		WithArgs(key).
		WillReturnRows(
			mock.NewRows([]string{
				"body",
			}).AddRow(
				body,
			),
		).RowsWillBeClosed()
	mock.ExpectRollback()

	if err := sqlDriver.Create(key, rel); err == nil {
		t.Fatalf("failed to create release with key %q: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlUpdate(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	mock.
		ExpectExec(regexp.QuoteMeta("UPDATE releases SET body=?, name=?, version=?, status=?, owner=?, modified_at=? WHERE key=?")).
		WithArgs(body, rel.Name, int(rel.Version), rspb.Status_Code_name[int32(rel.Info.Status.Code)], "TILLER", int(time.Now().Unix()), key).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := sqlDriver.Update(key, rel); err != nil {
		t.Fatalf("failed to update release with key %q: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlQuery(t *testing.T) {
	// Reflect actual use cases in ../storage.go
	labelSetDeployed := map[string]string{
		"NAME":   "smug-pigeon",
		"OWNER":  "TILLER",
		"STATUS": "DEPLOYED",
	}
	labelSetAll := map[string]string{
		"NAME":  "smug-pigeon",
		"OWNER": "TILLER",
	}

	supersededRelease := releaseStub("smug-pigeon", 1, "default", rspb.Status_SUPERSEDED)
	supersededReleaseBody, _ := encodeRelease(supersededRelease)
	deployedRelease := releaseStub("smug-pigeon", 2, "default", rspb.Status_DEPLOYED)
	deployedReleaseBody, _ := encodeRelease(deployedRelease)

	// Let's actually start our test
	sqlDriver, mock := newTestFixtureSQL(t)

	mock.
		ExpectQuery(regexp.QuoteMeta("SELECT body FROM releases WHERE name=? AND owner=? AND status=?")).
		WithArgs("smug-pigeon", "TILLER", "DEPLOYED").
		WillReturnRows(
			mock.NewRows([]string{
				"body",
			}).AddRow(
				deployedReleaseBody,
			),
		).RowsWillBeClosed()

	mock.
		ExpectQuery(regexp.QuoteMeta("SELECT body FROM releases WHERE name=? AND owner=?")).
		WithArgs("smug-pigeon", "TILLER").
		WillReturnRows(
			mock.NewRows([]string{
				"body",
			}).AddRow(
				supersededReleaseBody,
			).AddRow(
				deployedReleaseBody,
			),
		).RowsWillBeClosed()

	results, err := sqlDriver.Query(labelSetDeployed)
	if err != nil {
		t.Fatalf("failed to query for deployed smug-pigeon release: %v", err)
	}

	for _, res := range results {
		if !shallowReleaseEqual(res, deployedRelease) {
			t.Errorf("Expected release {%q}, got {%q}", deployedRelease, res)
		}
	}

	results, err = sqlDriver.Query(labelSetAll)
	if err != nil {
		t.Fatalf("failed to query release history for smug-pigeon: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected a resultset of size 2, got %d", len(results))
	}

	for _, res := range results {
		if !shallowReleaseEqual(res, deployedRelease) && !shallowReleaseEqual(res, supersededRelease) {
			t.Errorf("Expected release {%q} or {%q}, got {%q}", deployedRelease, supersededRelease, res)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlDelete(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	body, _ := encodeRelease(rel)

	sqlDriver, mock := newTestFixtureSQL(t)

	mock.ExpectBegin()
	mock.
		ExpectQuery("SELECT body FROM releases WHERE key = ?").
		WithArgs(key).
		WillReturnRows(
			mock.NewRows([]string{
				"body",
			}).AddRow(
				body,
			),
		).RowsWillBeClosed()

	mock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM releases WHERE key = $1")).
		WithArgs(key).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deletedRelease, err := sqlDriver.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete release with key %q: %v", key, err)
	}

	if !shallowReleaseEqual(rel, deletedRelease) {
		t.Errorf("Expected release {%q}, got {%q}", rel, deletedRelease)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}
