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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"

	sq "github.com/Masterminds/squirrel"

	// Import pq for postgres dialect
	_ "github.com/lib/pq"

	rspb "helm.sh/helm/v3/pkg/release"
)

var _ Driver = (*SQL)(nil)

var labelMap = map[string]struct{}{
	"modifiedAt": {},
	"createdAt":  {},
	"version":    {},
	"status":     {},
	"owner":      {},
	"name":       {},
}

const postgreSQLDialect = "postgres"

// SQLDriverName is the string name of this driver.
const SQLDriverName = "SQL"

const sqlReleaseTableName = "releases_v1"
const sqlCustomLabelsTableName = "custom_labels_v1"

const (
	sqlReleaseTableKeyColumn        = "key"
	sqlReleaseTableTypeColumn       = "type"
	sqlReleaseTableBodyColumn       = "body"
	sqlReleaseTableNameColumn       = "name"
	sqlReleaseTableNamespaceColumn  = "namespace"
	sqlReleaseTableVersionColumn    = "version"
	sqlReleaseTableStatusColumn     = "status"
	sqlReleaseTableOwnerColumn      = "owner"
	sqlReleaseTableCreatedAtColumn  = "createdAt"
	sqlReleaseTableModifiedAtColumn = "modifiedAt"

	sqlCustomLabelsTableReleaseKeyColumn       = "releaseKey"
	sqlCustomLabelsTableReleaseNamespaceColumn = "releaseNamespace"
	sqlCustomLabelsTableKeyColumn              = "key"
	sqlCustomLabelsTableValueColumn            = "value"
)

// Following limits based on k8s labels limits - https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
const (
	sqlCustomLabelsTableKeyMaxLenght   = 253 + 1 + 63
	sqlCustomLabelsTableValueMaxLenght = 63
)

const (
	sqlReleaseDefaultOwner = "helm"
	sqlReleaseDefaultType  = "helm.sh/release.v1"
)

// SQL is the sql storage driver implementation.
type SQL struct {
	db               *sqlx.DB
	namespace        string
	statementBuilder sq.StatementBuilderType

	Log func(string, ...interface{})
}

// Name returns the name of the driver.
func (s *SQL) Name() string {
	return SQLDriverName
}

// Check if all migrations al
func (s *SQL) checkAlreadyApplied(migrations []*migrate.Migration) bool {
	// make map (set) of ids for fast search
	migrationsIds := make(map[string]struct{})
	for _, migration := range migrations {
		migrationsIds[migration.Id] = struct{}{}
	}

	// get list of applied migrations
	migrate.SetDisableCreateTable(true)
	records, err := migrate.GetMigrationRecords(s.db.DB, postgreSQLDialect)
	migrate.SetDisableCreateTable(false)
	if err != nil {
		s.Log("checkAlreadyApplied: failed to get migration records: %v", err)
		return false
	}

	for _, record := range records {
		if _, ok := migrationsIds[record.Id]; ok {
			s.Log("checkAlreadyApplied: found previous migration (Id: %v) applied at %v", record.Id, record.AppliedAt)
			delete(migrationsIds, record.Id)
		}
	}

	// check if all migrations appliyed
	if len(migrationsIds) != 0 {
		for id := range migrationsIds {
			s.Log("checkAlreadyApplied: find unapplied migration (id: %v)", id)
		}
		return false
	}
	return true
}

func (s *SQL) ensureDBSetup() error {

	migrations := &migrate.MemoryMigrationSource{
		Migrations: []*migrate.Migration{
			{
				Id: "init",
				Up: []string{
					fmt.Sprintf(`
						CREATE TABLE %s (
							%s VARCHAR(90),
							%s VARCHAR(64) NOT NULL,
							%s TEXT NOT NULL,
							%s VARCHAR(64) NOT NULL,
							%s VARCHAR(64) NOT NULL,
							%s INTEGER NOT NULL,
							%s TEXT NOT NULL,
							%s TEXT NOT NULL,
							%s INTEGER NOT NULL,
							%s INTEGER NOT NULL DEFAULT 0,
							PRIMARY KEY(%s, %s)
						);
						CREATE INDEX ON %s (%s, %s);
						CREATE INDEX ON %s (%s);
						CREATE INDEX ON %s (%s);
						CREATE INDEX ON %s (%s);
						CREATE INDEX ON %s (%s);
						CREATE INDEX ON %s (%s);
	
						GRANT ALL ON %s TO PUBLIC;
	
						ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
					`,
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
						sqlReleaseTableModifiedAtColumn,
						sqlReleaseTableKeyColumn,
						sqlReleaseTableNamespaceColumn,
						sqlReleaseTableName,
						sqlReleaseTableKeyColumn,
						sqlReleaseTableNamespaceColumn,
						sqlReleaseTableName,
						sqlReleaseTableVersionColumn,
						sqlReleaseTableName,
						sqlReleaseTableStatusColumn,
						sqlReleaseTableName,
						sqlReleaseTableOwnerColumn,
						sqlReleaseTableName,
						sqlReleaseTableCreatedAtColumn,
						sqlReleaseTableName,
						sqlReleaseTableModifiedAtColumn,
						sqlReleaseTableName,
						sqlReleaseTableName,
					),
				},
				Down: []string{
					fmt.Sprintf(`
						DROP TABLE %s;
					`, sqlReleaseTableName),
				},
			},
			{
				Id: "custom_labels",
				Up: []string{
					fmt.Sprintf(`
						CREATE TABLE %s (
							%s VARCHAR(64),
							%s VARCHAR(67),
							%s VARCHAR(%d), 
							%s VARCHAR(%d)
						);
						CREATE INDEX ON %s (%s, %s);
						
						GRANT ALL ON %s TO PUBLIC;
						ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
					`,
						sqlCustomLabelsTableName,
						sqlCustomLabelsTableReleaseKeyColumn,
						sqlCustomLabelsTableReleaseNamespaceColumn,
						sqlCustomLabelsTableKeyColumn,
						sqlCustomLabelsTableKeyMaxLenght,
						sqlCustomLabelsTableValueColumn,
						sqlCustomLabelsTableValueMaxLenght,
						sqlCustomLabelsTableName,
						sqlCustomLabelsTableReleaseKeyColumn,
						sqlCustomLabelsTableReleaseNamespaceColumn,
						sqlCustomLabelsTableName,
						sqlCustomLabelsTableName,
					),
				},
				Down: []string{
					fmt.Sprintf(`
						DELETE TABLE %s;
					`, sqlCustomLabelsTableName),
				},
			},
		},
	}

	// Check that init migration already applied
	if s.checkAlreadyApplied(migrations.Migrations) {
		return nil
	}

	// Populate the database with the relations we need if they don't exist yet
	_, err := migrate.Exec(s.db.DB, postgreSQLDialect, migrations, migrate.Up)
	return err
}

// SQLReleaseWrapper describes how Helm releases are stored in an SQL database
type SQLReleaseWrapper struct {
	// The primary key, made of {release-name}.{release-version}
	Key string `db:"key"`

	// See https://github.com/helm/helm/blob/c9fe3d118caec699eb2565df9838673af379ce12/pkg/storage/driver/secrets.go#L231
	Type string `db:"type"`

	// The rspb.Release body, as a base64-encoded string
	Body string `db:"body"`

	// Release "labels" that can be used as filters in the storage.Query(labels map[string]string)
	// we implemented. Note that allowing Helm users to filter against new dimensions will require a
	// new migration to be added, and the Create and/or update functions to be updated accordingly.
	Name       string `db:"name"`
	Namespace  string `db:"namespace"`
	Version    int    `db:"version"`
	Status     string `db:"status"`
	Owner      string `db:"owner"`
	CreatedAt  int    `db:"createdAt"`
	ModifiedAt int    `db:"modifiedAt"`
}

type SQLReleaseCustomLabelWrapper struct {
	ReleaseKey       string `db:"release_key"`
	ReleaseNamespace string `db:"release_namespace"`
	Key              string `db:"key"`
	Value            string `db:"value"`
}

// NewSQL initializes a new sql driver.
func NewSQL(connectionString string, logger func(string, ...interface{}), namespace string) (*SQL, error) {
	db, err := sqlx.Connect(postgreSQLDialect, connectionString)
	if err != nil {
		return nil, err
	}

	driver := &SQL{
		db:               db,
		Log:              logger,
		statementBuilder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	if err := driver.ensureDBSetup(); err != nil {
		return nil, err
	}

	driver.namespace = namespace

	return driver, nil
}

// Get returns the release named by key.
func (s *SQL) Get(key string) (*rspb.Release, error) {
	var record SQLReleaseWrapper

	qb := s.statementBuilder.
		Select(sqlReleaseTableBodyColumn).
		From(sqlReleaseTableName).
		Where(sq.Eq{sqlReleaseTableKeyColumn: key}).
		Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace})

	query, args, err := qb.ToSql()
	if err != nil {
		s.Log("failed to build query: %v", err)
		return nil, err
	}

	// Get will return an error if the result is empty
	if err := s.db.Get(&record, query, args...); err != nil {
		s.Log("got SQL error when getting release %s: %v", key, err)
		return nil, ErrReleaseNotFound
	}

	release, err := decodeRelease(record.Body)
	if err != nil {
		s.Log("get: failed to decode data %q: %v", key, err)
		return nil, err
	}

	if release.Labels, err = s.getReleaseCustomLabels(key, s.namespace); err != nil {
		s.Log("failed to get release %s/%s custom labels: %v", s.namespace, key, err)
		return nil, err
	}

	return release, nil
}

// List returns the list of all releases such that filter(release) == true
func (s *SQL) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	sb := s.statementBuilder.
		Select(sqlReleaseTableKeyColumn, sqlReleaseTableNamespaceColumn, sqlReleaseTableBodyColumn).
		From(sqlReleaseTableName).
		Where(sq.Eq{sqlReleaseTableOwnerColumn: sqlReleaseDefaultOwner})

	// If a namespace was specified, we only list releases from that namespace
	if s.namespace != "" {
		sb = sb.Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace})
	}

	query, args, err := sb.ToSql()
	if err != nil {
		s.Log("failed to build query: %v", err)
		return nil, err
	}

	var records = []SQLReleaseWrapper{}
	if err := s.db.Select(&records, query, args...); err != nil {
		s.Log("list: failed to list: %v", err)
		return nil, err
	}

	var releases []*rspb.Release
	for _, record := range records {
		release, err := decodeRelease(record.Body)
		if err != nil {
			s.Log("list: failed to decode release: %v: %v", record, err)
			continue
		}

		if release.Labels, err = s.getReleaseCustomLabels(record.Key, record.Namespace); err != nil {
			s.Log("failed to get release %s/%s custom labels: %v", record.Namespace, record.Key, err)
			return nil, err
		}
		for k, v := range getReleaseSystemLabels(release) {
			release.Labels[k] = v
		}

		if filter(release) {
			releases = append(releases, release)
		}
	}

	return releases, nil
}

// Query returns the set of releases that match the provided set of labels.
func (s *SQL) Query(labels map[string]string) ([]*rspb.Release, error) {
	sb := s.statementBuilder.
		Select(sqlReleaseTableKeyColumn, sqlReleaseTableNamespaceColumn, sqlReleaseTableBodyColumn).
		From(sqlReleaseTableName)

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := labelMap[key]; ok {
			sb = sb.Where(sq.Eq{key: labels[key]})
		} else {
			s.Log("unknown label %s", key)
			return nil, fmt.Errorf("unknown label %s", key)
		}
	}

	// If a namespace was specified, we only list releases from that namespace
	if s.namespace != "" {
		sb = sb.Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace})
	}

	// Build our query
	query, args, err := sb.ToSql()
	if err != nil {
		s.Log("failed to build query: %v", err)
		return nil, err
	}

	var records = []SQLReleaseWrapper{}
	if err := s.db.Select(&records, query, args...); err != nil {
		s.Log("list: failed to query with labels: %v", err)
		return nil, err
	}

	if len(records) == 0 {
		return nil, ErrReleaseNotFound
	}

	var releases []*rspb.Release
	for _, record := range records {
		release, err := decodeRelease(record.Body)
		if err != nil {
			s.Log("list: failed to decode release: %v: %v", record, err)
			continue
		}

		if release.Labels, err = s.getReleaseCustomLabels(record.Key, record.Namespace); err != nil {
			s.Log("failed to get release %s/%s custom labels: %v", record.Namespace, record.Key, err)
			return nil, err
		}

		releases = append(releases, release)
	}

	if len(releases) == 0 {
		return nil, ErrReleaseNotFound
	}

	return releases, nil
}

// Create creates a new release.
func (s *SQL) Create(key string, rls *rspb.Release) error {
	namespace := rls.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	s.namespace = namespace

	body, err := encodeRelease(rls)
	if err != nil {
		s.Log("failed to encode release: %v", err)
		return err
	}

	transaction, err := s.db.Beginx()
	if err != nil {
		s.Log("failed to start SQL transaction: %v", err)
		return fmt.Errorf("error beginning transaction: %v", err)
	}

	insertQuery, args, err := s.statementBuilder.
		Insert(sqlReleaseTableName).
		Columns(
			sqlReleaseTableKeyColumn,
			sqlReleaseTableTypeColumn,
			sqlReleaseTableBodyColumn,
			sqlReleaseTableNameColumn,
			sqlReleaseTableNamespaceColumn,
			sqlReleaseTableVersionColumn,
			sqlReleaseTableStatusColumn,
			sqlReleaseTableOwnerColumn,
			sqlReleaseTableCreatedAtColumn,
		).
		Values(
			key,
			sqlReleaseDefaultType,
			body,
			rls.Name,
			namespace,
			int(rls.Version),
			rls.Info.Status.String(),
			sqlReleaseDefaultOwner,
			int(time.Now().Unix()),
		).ToSql()
	if err != nil {
		s.Log("failed to build insert query: %v", err)
		return err
	}

	if _, err := transaction.Exec(insertQuery, args...); err != nil {
		defer transaction.Rollback()

		selectQuery, args, buildErr := s.statementBuilder.
			Select(sqlReleaseTableKeyColumn).
			From(sqlReleaseTableName).
			Where(sq.Eq{sqlReleaseTableKeyColumn: key}).
			Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace}).
			ToSql()
		if buildErr != nil {
			s.Log("failed to build select query: %v", buildErr)
			return err
		}

		var record SQLReleaseWrapper
		if err := transaction.Get(&record, selectQuery, args...); err == nil {
			s.Log("release %s already exists", key)
			return ErrReleaseExists
		}

		s.Log("failed to store release %s in SQL database: %v", key, err)
		return err
	}

	// Filtering labels before insert cause in SQL storage driver system releases are stored in separate columns of release table
	for k, v := range filterSystemLabels(rls.Labels) {
		insertLabelsQuery, args, err := s.statementBuilder.
			Insert(sqlCustomLabelsTableName).
			Columns(
				sqlCustomLabelsTableReleaseKeyColumn,
				sqlCustomLabelsTableReleaseNamespaceColumn,
				sqlCustomLabelsTableKeyColumn,
				sqlCustomLabelsTableValueColumn,
			).
			Values(
				key,
				namespace,
				k,
				v,
			).ToSql()

		if err != nil {
			defer transaction.Rollback()
			s.Log("failed to build insert query: %v", err)
			return err
		}

		if _, err := transaction.Exec(insertLabelsQuery, args...); err != nil {
			defer transaction.Rollback()
			s.Log("failed to write Labels: %v", err)
			return err
		}
	}
	defer transaction.Commit()

	return nil
}

// Update updates a release.
func (s *SQL) Update(key string, rls *rspb.Release) error {
	namespace := rls.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	s.namespace = namespace

	body, err := encodeRelease(rls)
	if err != nil {
		s.Log("failed to encode release: %v", err)
		return err
	}

	query, args, err := s.statementBuilder.
		Update(sqlReleaseTableName).
		Set(sqlReleaseTableBodyColumn, body).
		Set(sqlReleaseTableNameColumn, rls.Name).
		Set(sqlReleaseTableVersionColumn, int(rls.Version)).
		Set(sqlReleaseTableStatusColumn, rls.Info.Status.String()).
		Set(sqlReleaseTableOwnerColumn, sqlReleaseDefaultOwner).
		Set(sqlReleaseTableModifiedAtColumn, int(time.Now().Unix())).
		Where(sq.Eq{sqlReleaseTableKeyColumn: key}).
		Where(sq.Eq{sqlReleaseTableNamespaceColumn: namespace}).
		ToSql()

	if err != nil {
		s.Log("failed to build update query: %v", err)
		return err
	}

	if _, err := s.db.Exec(query, args...); err != nil {
		s.Log("failed to update release %s in SQL database: %v", key, err)
		return err
	}

	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (s *SQL) Delete(key string) (*rspb.Release, error) {
	transaction, err := s.db.Beginx()
	if err != nil {
		s.Log("failed to start SQL transaction: %v", err)
		return nil, fmt.Errorf("error beginning transaction: %v", err)
	}

	selectQuery, args, err := s.statementBuilder.
		Select(sqlReleaseTableBodyColumn).
		From(sqlReleaseTableName).
		Where(sq.Eq{sqlReleaseTableKeyColumn: key}).
		Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace}).
		ToSql()
	if err != nil {
		s.Log("failed to build select query: %v", err)
		return nil, err
	}

	var record SQLReleaseWrapper
	err = transaction.Get(&record, selectQuery, args...)
	if err != nil {
		s.Log("release %s not found: %v", key, err)
		return nil, ErrReleaseNotFound
	}

	release, err := decodeRelease(record.Body)
	if err != nil {
		s.Log("failed to decode release %s: %v", key, err)
		transaction.Rollback()
		return nil, err
	}
	defer transaction.Commit()

	deleteQuery, args, err := s.statementBuilder.
		Delete(sqlReleaseTableName).
		Where(sq.Eq{sqlReleaseTableKeyColumn: key}).
		Where(sq.Eq{sqlReleaseTableNamespaceColumn: s.namespace}).
		ToSql()
	if err != nil {
		s.Log("failed to build delete query: %v", err)
		return nil, err
	}

	_, err = transaction.Exec(deleteQuery, args...)
	if err != nil {
		s.Log("failed perform delete query: %v", err)
		return release, err
	}

	if release.Labels, err = s.getReleaseCustomLabels(key, s.namespace); err != nil {
		s.Log("failed to get release %s/%s custom labels: %v", s.namespace, key, err)
		return nil, err
	}

	deleteCustomLabelsQuery, args, err := s.statementBuilder.
		Delete(sqlCustomLabelsTableName).
		Where(sq.Eq{sqlCustomLabelsTableReleaseKeyColumn: key}).
		Where(sq.Eq{sqlCustomLabelsTableReleaseNamespaceColumn: s.namespace}).
		ToSql()

	if err != nil {
		s.Log("failed to build delete Labels query: %v", err)
		return nil, err
	}
	_, err = transaction.Exec(deleteCustomLabelsQuery, args...)
	return release, err
}

// Get release custom labels from database
func (s *SQL) getReleaseCustomLabels(key string, namespace string) (map[string]string, error) {
	query, args, err := s.statementBuilder.
		Select(sqlCustomLabelsTableKeyColumn, sqlCustomLabelsTableValueColumn).
		From(sqlCustomLabelsTableName).
		Where(sq.Eq{sqlCustomLabelsTableReleaseKeyColumn: key,
			sqlCustomLabelsTableReleaseNamespaceColumn: s.namespace}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var labelsList = []SQLReleaseCustomLabelWrapper{}
	if err := s.db.Select(&labelsList, query, args...); err != nil {
		return nil, err
	}

	labelsMap := make(map[string]string)
	for _, i := range labelsList {
		labelsMap[i.Key] = i.Value
	}

	return filterSystemLabels(labelsMap), nil
}

// Rebuild system labels from release object
func getReleaseSystemLabels(rls *rspb.Release) map[string]string {
	return map[string]string{
		"name":    rls.Name,
		"owner":   sqlReleaseDefaultOwner,
		"status":  rls.Info.Status.String(),
		"version": strconv.Itoa(rls.Version),
	}
}
