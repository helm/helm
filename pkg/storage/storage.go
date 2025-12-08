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
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"helm.sh/helm/v4/internal/logging"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
	relutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// HelmStorageType is the type field of the Kubernetes storage object which stores the Helm release
// version. It is modified slightly replacing the '/': sh.helm/release.v1
// Note: The version 'v1' is incremented if the release object metadata is
// modified between major releases.
// This constant is used as a prefix for the Kubernetes storage object name.
const HelmStorageType = "sh.helm.release.v1"

// Storage represents a storage engine for a Release.
type Storage struct {
	driver.Driver

	// MaxHistory specifies the maximum number of historical releases that will
	// be retained, including the most recent release. Values of 0 or less are
	// ignored (meaning no limits are imposed).
	MaxHistory int

	// Embed a LogHolder to provide logger functionality
	logging.LogHolder
}

// Get retrieves the release from storage. An error is returned
// if the storage driver failed to fetch the release, or the
// release identified by the key, version pair does not exist.
func (s *Storage) Get(name string, version int) (release.Releaser, error) {
	s.Logger().Debug("getting release", "key", makeKey(name, version))
	return s.Driver.Get(makeKey(name, version))
}

// Create creates a new storage entry holding the release. An
// error is returned if the storage driver fails to store the
// release, or a release with an identical key already exists.
func (s *Storage) Create(rls release.Releaser) error {
	rac, err := release.NewAccessor(rls)
	if err != nil {
		return err
	}
	s.Logger().Debug("creating release", "key", makeKey(rac.Name(), rac.Version()))
	if s.MaxHistory > 0 {
		// Want to make space for one more release.
		if err := s.removeLeastRecent(rac.Name(), s.MaxHistory-1); err != nil &&
			!errors.Is(err, driver.ErrReleaseNotFound) {
			return err
		}
	}
	return s.Driver.Create(makeKey(rac.Name(), rac.Version()), rls)
}

// Update updates the release in storage. An error is returned if the
// storage backend fails to update the release or if the release
// does not exist.
func (s *Storage) Update(rls release.Releaser) error {
	rac, err := release.NewAccessor(rls)
	if err != nil {
		return err
	}
	s.Logger().Debug("updating release", "key", makeKey(rac.Name(), rac.Version()))
	return s.Driver.Update(makeKey(rac.Name(), rac.Version()), rls)
}

// Delete deletes the release from storage. An error is returned if
// the storage backend fails to delete the release or if the release
// does not exist.
func (s *Storage) Delete(name string, version int) (release.Releaser, error) {
	s.Logger().Debug("deleting release", "key", makeKey(name, version))
	return s.Driver.Delete(makeKey(name, version))
}

// ListReleases returns all releases from storage. An error is returned if the
// storage backend fails to retrieve the releases.
func (s *Storage) ListReleases() ([]release.Releaser, error) {
	s.Logger().Debug("listing all releases in storage")
	return s.List(func(_ release.Releaser) bool { return true })
}

// releaserToV1Release is a helper function to convert a v1 release passed by interface
// into the type object.
func releaserToV1Release(rel release.Releaser) (*rspb.Release, error) {
	switch r := rel.(type) {
	case rspb.Release:
		return &r, nil
	case *rspb.Release:
		return r, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported release type: %T", rel)
	}
}

// ListUninstalled returns all releases with Status == UNINSTALLED. An error is returned
// if the storage backend fails to retrieve the releases.
func (s *Storage) ListUninstalled() ([]release.Releaser, error) {
	s.Logger().Debug("listing uninstalled releases in storage")
	return s.List(func(rls release.Releaser) bool {
		rel, err := releaserToV1Release(rls)
		if err != nil {
			// This will only happen if calling code does not pass the proper types. This is
			// a problem with the application and not user data.
			s.Logger().Error("unable to convert release to typed release", slog.Any("error", err))
			panic(fmt.Sprintf("unable to convert release to typed release: %s", err))
		}
		return relutil.StatusFilter(common.StatusUninstalled).Check(rel)
	})
}

// ListDeployed returns all releases with Status == DEPLOYED. An error is returned
// if the storage backend fails to retrieve the releases.
func (s *Storage) ListDeployed() ([]release.Releaser, error) {
	s.Logger().Debug("listing all deployed releases in storage")
	return s.List(func(rls release.Releaser) bool {
		rel, err := releaserToV1Release(rls)
		if err != nil {
			// This will only happen if calling code does not pass the proper types. This is
			// a problem with the application and not user data.
			s.Logger().Error("unable to convert release to typed release", slog.Any("error", err))
			panic(fmt.Sprintf("unable to convert release to typed release: %s", err))
		}
		return relutil.StatusFilter(common.StatusDeployed).Check(rel)
	})
}

// Deployed returns the last deployed release with the provided release name, or
// returns driver.NewErrNoDeployedReleases if not found.
func (s *Storage) Deployed(name string) (release.Releaser, error) {
	ls, err := s.DeployedAll(name)
	if err != nil {
		return nil, err
	}

	if len(ls) == 0 {
		return nil, driver.NewErrNoDeployedReleases(name)
	}

	rls, err := releaseListToV1List(ls)
	if err != nil {
		return nil, err
	}

	// If executed concurrently, Helm's database gets corrupted
	// and multiple releases are DEPLOYED. Take the latest.
	relutil.Reverse(rls, relutil.SortByRevision)

	return rls[0], nil
}

func releaseListToV1List(ls []release.Releaser) ([]*rspb.Release, error) {
	rls := make([]*rspb.Release, 0, len(ls))
	for _, val := range ls {
		rel, err := releaserToV1Release(val)
		if err != nil {
			return nil, err
		}
		rls = append(rls, rel)
	}

	return rls, nil
}

// DeployedAll returns all deployed releases with the provided name, or
// returns driver.NewErrNoDeployedReleases if not found.
func (s *Storage) DeployedAll(name string) ([]release.Releaser, error) {
	s.Logger().Debug("getting deployed releases", "name", name)

	ls, err := s.Query(map[string]string{
		"name":   name,
		"owner":  "helm",
		"status": "deployed",
	})
	if err == nil {
		return ls, nil
	}
	if strings.Contains(err.Error(), "not found") {
		return nil, driver.NewErrNoDeployedReleases(name)
	}
	return nil, err
}

// History returns the revision history for the release with the provided name, or
// returns driver.ErrReleaseNotFound if no such release name exists.
func (s *Storage) History(name string) ([]release.Releaser, error) {
	s.Logger().Debug("getting release history", "name", name)

	return s.Query(map[string]string{"name": name, "owner": "helm"})
}

// removeLeastRecent removes items from history until the length number of releases
// does not exceed max.
//
// We allow max to be set explicitly so that calling functions can "make space"
// for the new records they are going to write.
func (s *Storage) removeLeastRecent(name string, maximum int) error {
	if maximum < 0 {
		return nil
	}
	h, err := s.History(name)
	if err != nil {
		return err
	}
	if len(h) <= maximum {
		return nil
	}
	rls, err := releaseListToV1List(h)
	if err != nil {
		return err
	}

	// We want oldest to newest
	relutil.SortByRevision(rls)

	lastDeployed, err := s.Deployed(name)
	if err != nil && !errors.Is(err, driver.ErrNoDeployedReleases) {
		return err
	}

	var toDelete []release.Releaser
	for _, rel := range rls {
		// once we have enough releases to delete to reach the maximum, stop
		if len(rls)-len(toDelete) == maximum {
			break
		}
		if lastDeployed != nil {
			ldac, err := release.NewAccessor(lastDeployed)
			if err != nil {
				return err
			}
			if rel.Version != ldac.Version() {
				toDelete = append(toDelete, rel)
			}
		} else {
			toDelete = append(toDelete, rel)
		}
	}

	// Delete as many as possible. In the case of API throughput limitations,
	// multiple invocations of this function will eventually delete them all.
	errs := []error{}
	for _, rel := range toDelete {
		rac, err := release.NewAccessor(rel)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		err = s.deleteReleaseVersion(name, rac.Version())
		if err != nil {
			errs = append(errs, err)
		}
	}

	s.Logger().Debug("pruned records", "count", len(toDelete), "release", name, "errors", len(errs))
	switch c := len(errs); c {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("encountered %d deletion errors. First is: %w", c, errs[0])
	}
}

func (s *Storage) deleteReleaseVersion(name string, version int) error {
	key := makeKey(name, version)
	_, err := s.Delete(name, version)
	if err != nil {
		s.Logger().Debug("error pruning release", slog.String("key", key), slog.Any("error", err))
		return err
	}
	return nil
}

// Last fetches the last revision of the named release.
func (s *Storage) Last(name string) (release.Releaser, error) {
	s.Logger().Debug("getting last revision", "name", name)
	h, err := s.History(name)
	if err != nil {
		return nil, err
	}
	if len(h) == 0 {
		return nil, fmt.Errorf("no revision for release %q", name)
	}
	rls, err := releaseListToV1List(h)
	if err != nil {
		return nil, err
	}

	relutil.Reverse(rls, relutil.SortByRevision)
	return rls[0], nil
}

// makeKey concatenates the Kubernetes storage object type, a release name and version
// into a string with format:```<helm_storage_type>.<release_name>.v<release_version>```.
// The storage type is prepended to keep name uniqueness between different
// release storage types. An example of clash when not using the type:
// https://github.com/helm/helm/issues/6435.
// This key is used to uniquely identify storage objects.
func makeKey(rlsname string, version int) string {
	return fmt.Sprintf("%s.%s.v%d", HelmStorageType, rlsname, version)
}

// Init initializes a new storage backend with the driver d.
// If d is nil, the default in-memory driver is used.
func Init(d driver.Driver) *Storage {
	// default driver is in memory
	if d == nil {
		d = driver.NewMemory()
	}
	s := &Storage{
		Driver: d,
	}

	// Get logger from driver if it implements the LoggerSetterGetter interface
	if ls, ok := d.(logging.LoggerSetterGetter); ok {
		ls.SetLogger(s.Logger().Handler())
	} else {
		// If the driver does not implement the LoggerSetterGetter interface, set the default logger
		s.SetLogger(slog.Default().Handler())
	}
	return s
}
