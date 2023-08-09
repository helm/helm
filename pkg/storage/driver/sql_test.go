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
	"reflect"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	rspb "helm.sh/helm/v3/pkg/release"
)

func TestSQLName(t *testing.T) {
	sqlDriver, _ := newTestFixtureSQL(t)
	if sqlDriver.Name() != SQLDriverName {
		t.Errorf("Expected name to be %s, got %s", SQLDriverName, sqlDriver.Name())
	}
}

func TestSQLGet(t *testing.T) {
	vers := int(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	body, _ := encodeRelease(rel)

	sqlDriver, mock := newTestFixtureSQL(t)

	query := fmt.Sprintf(
		regexp.QuoteMeta("SELECT %s FROM %s WHERE %s = $1 AND %s = $2"),
		sqlReleaseTableBodyColumn,
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.
		ExpectQuery(query).
		WithArgs(key, namespace).
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableBodyColumn,
			}).AddRow(
				body,
			),
		).RowsWillBeClosed()

	got, err := sqlDriver.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %v", err)
	}

	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected release {%v}, got {%v}", rel, got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSQLList(t *testing.T) {
	body1, _ := encodeRelease(releaseStub("key-1", 1, "default", rspb.StatusUninstalled))
	body2, _ := encodeRelease(releaseStub("key-2", 1, "default", rspb.StatusUninstalled))
	body3, _ := encodeRelease(releaseStub("key-3", 1, "default", rspb.StatusDeployed))
	body4, _ := encodeRelease(releaseStub("key-4", 1, "default", rspb.StatusDeployed))
	body5, _ := encodeRelease(releaseStub("key-5", 1, "default", rspb.StatusSuperseded))
	body6, _ := encodeRelease(releaseStub("key-6", 1, "default", rspb.StatusSuperseded))

	sqlDriver, mock := newTestFixtureSQL(t)

	for i := 0; i < 3; i++ {
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE %s = $1 AND %s = $2",
			sqlReleaseTableBodyColumn,
			sqlReleaseTableName,
			sqlReleaseTableOwnerColumn,
			sqlReleaseTableNamespaceColumn,
		)

		mock.
			ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(sqlReleaseDefaultOwner, sqlDriver.namespace).
			WillReturnRows(
				mock.NewRows([]string{
					sqlReleaseTableBodyColumn,
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
		return rel.Info.Status == rspb.StatusUninstalled
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
		return rel.Info.Status == rspb.StatusDeployed
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
		return rel.Info.Status == rspb.StatusSuperseded
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
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	query := fmt.Sprintf(
		"INSERT INTO %s (%s,%s,%s,%s,%s,%s,%s,%s,%s) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableTypeColumn,
		sqlReleaseTableBodyColumn,
		sqlReleaseTableNameColumn,
		sqlReleaseTableNamespaceColumn,
		sqlReleaseTableVersionColumn,
		sqlReleaseTableStatusColumn,
		sqlReleaseTableOwnerColumn,
		sqlReleaseTableCreatedAtColumn,
	)

	mock.ExpectBegin()
	mock.
		ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(key, sqlReleaseDefaultType, body, rel.Name, rel.Namespace, int(rel.Version), rel.Info.Status.String(), sqlReleaseDefaultOwner, int(time.Now().Unix())).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := sqlDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlCreateAlreadyExists(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (%s,%s,%s,%s,%s,%s,%s,%s,%s) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableTypeColumn,
		sqlReleaseTableBodyColumn,
		sqlReleaseTableNameColumn,
		sqlReleaseTableNamespaceColumn,
		sqlReleaseTableVersionColumn,
		sqlReleaseTableStatusColumn,
		sqlReleaseTableOwnerColumn,
		sqlReleaseTableCreatedAtColumn,
	)

	// Insert fails (primary key already exists)
	mock.ExpectBegin()
	mock.
		ExpectExec(regexp.QuoteMeta(insertQuery)).
		WithArgs(key, sqlReleaseDefaultType, body, rel.Name, rel.Namespace, int(rel.Version), rel.Info.Status.String(), sqlReleaseDefaultOwner, int(time.Now().Unix())).
		WillReturnError(fmt.Errorf("dialect dependent SQL error"))

	selectQuery := fmt.Sprintf(
		regexp.QuoteMeta("SELECT %s FROM %s WHERE %s = $1 AND %s = $2"),
		sqlReleaseTableKeyColumn,
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableNamespaceColumn,
	)

	// Let's check that we do make sure the error is due to a release already existing
	mock.
		ExpectQuery(selectQuery).
		WithArgs(key, namespace).
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableKeyColumn,
			}).AddRow(
				key,
			),
		).RowsWillBeClosed()
	mock.ExpectRollback()

	if err := sqlDriver.Create(key, rel); err == nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlUpdate(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	sqlDriver, mock := newTestFixtureSQL(t)
	body, _ := encodeRelease(rel)

	query := fmt.Sprintf(
		"UPDATE %s SET %s = $1, %s = $2, %s = $3, %s = $4, %s = $5, %s = $6 WHERE %s = $7 AND %s = $8",
		sqlReleaseTableName,
		sqlReleaseTableBodyColumn,
		sqlReleaseTableNameColumn,
		sqlReleaseTableVersionColumn,
		sqlReleaseTableStatusColumn,
		sqlReleaseTableOwnerColumn,
		sqlReleaseTableModifiedAtColumn,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.
		ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(body, rel.Name, int(rel.Version), rel.Info.Status.String(), sqlReleaseDefaultOwner, int(time.Now().Unix()), key, namespace).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := sqlDriver.Update(key, rel); err != nil {
		t.Fatalf("failed to update release with key %s: %v", key, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlQuery(t *testing.T) {
	// Reflect actual use cases in ../storage.go
	labelSetUnknown := map[string]string{
		"name":   "smug-pigeon",
		"owner":  sqlReleaseDefaultOwner,
		"status": "unknown",
	}
	labelSetDeployed := map[string]string{
		"name":   "smug-pigeon",
		"owner":  sqlReleaseDefaultOwner,
		"status": "deployed",
	}
	labelSetAll := map[string]string{
		"name":  "smug-pigeon",
		"owner": sqlReleaseDefaultOwner,
	}

	supersededRelease := releaseStub("smug-pigeon", 1, "default", rspb.StatusSuperseded)
	supersededReleaseBody, _ := encodeRelease(supersededRelease)
	deployedRelease := releaseStub("smug-pigeon", 2, "default", rspb.StatusDeployed)
	deployedReleaseBody, _ := encodeRelease(deployedRelease)

	// Let's actually start our test
	sqlDriver, mock := newTestFixtureSQL(t)

	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s = $2 AND %s = $3 AND %s = $4",
		sqlReleaseTableBodyColumn,
		sqlReleaseTableName,
		sqlReleaseTableNameColumn,
		sqlReleaseTableOwnerColumn,
		sqlReleaseTableStatusColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.
		ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("smug-pigeon", sqlReleaseDefaultOwner, "unknown", "default").
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableBodyColumn,
			}),
		).RowsWillBeClosed()

	mock.
		ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("smug-pigeon", sqlReleaseDefaultOwner, "deployed", "default").
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableBodyColumn,
			}).AddRow(
				deployedReleaseBody,
			),
		).RowsWillBeClosed()

	query = fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s = $2 AND %s = $3",
		sqlReleaseTableBodyColumn,
		sqlReleaseTableName,
		sqlReleaseTableNameColumn,
		sqlReleaseTableOwnerColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.
		ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("smug-pigeon", sqlReleaseDefaultOwner, "default").
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableBodyColumn,
			}).AddRow(
				supersededReleaseBody,
			).AddRow(
				deployedReleaseBody,
			),
		).RowsWillBeClosed()

	_, err := sqlDriver.Query(labelSetUnknown)
	if err == nil {
		t.Errorf("Expected error {%v}, got nil", ErrReleaseNotFound)
	} else if err != ErrReleaseNotFound {
		t.Fatalf("failed to query for unknown smug-pigeon release: %v", err)
	}

	results, err := sqlDriver.Query(labelSetDeployed)
	if err != nil {
		t.Fatalf("failed to query for deployed smug-pigeon release: %v", err)
	}

	for _, res := range results {
		if !reflect.DeepEqual(res, deployedRelease) {
			t.Errorf("Expected release {%v}, got {%v}", deployedRelease, res)
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
		if !reflect.DeepEqual(res, deployedRelease) && !reflect.DeepEqual(res, supersededRelease) {
			t.Errorf("Expected release {%v} or {%v}, got {%v}", deployedRelease, supersededRelease, res)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
}

func TestSqlDelete(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	body, _ := encodeRelease(rel)

	sqlDriver, mock := newTestFixtureSQL(t)

	selectQuery := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s = $2",
		sqlReleaseTableBodyColumn,
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.ExpectBegin()
	mock.
		ExpectQuery(regexp.QuoteMeta(selectQuery)).
		WithArgs(key, namespace).
		WillReturnRows(
			mock.NewRows([]string{
				sqlReleaseTableBodyColumn,
			}).AddRow(
				body,
			),
		).RowsWillBeClosed()

	deleteQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = $1 AND %s = $2",
		sqlReleaseTableName,
		sqlReleaseTableKeyColumn,
		sqlReleaseTableNamespaceColumn,
	)

	mock.
		ExpectExec(regexp.QuoteMeta(deleteQuery)).
		WithArgs(key, namespace).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deletedRelease, err := sqlDriver.Delete(key)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sql expectations weren't met: %v", err)
	}
	if err != nil {
		t.Fatalf("failed to delete release with key %q: %v", key, err)
	}

	if !reflect.DeepEqual(rel, deletedRelease) {
		t.Errorf("Expected release {%v}, got {%v}", rel, deletedRelease)
	}
}
