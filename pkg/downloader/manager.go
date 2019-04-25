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

package downloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/resolver"
)

// Manager handles the lifecycle of fetching, resolving, and storing dependencies.
type Manager struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// ChartPath is the path to the unpacked base chart upon which this operates.
	ChartPath string
	// Debug is the global "--debug" flag
	Debug bool
	// Client for the operation
	Client *repo.Client
}

// Build rebuilds a local charts directory from a lockfile.
//
// If the lockfile is not present, this will run a Manager.Update()
//
// If SkipUpdate is set, this will not update the repository.
func (m *Manager) Build() error {
	c, err := m.loadChartDir()
	if err != nil {
		return err
	}

	// If a lock file is found, run a build from that. Otherwise, just do
	// an update.
	lock := c.Lock
	if lock == nil {
		return m.Update()
	}

	req := c.Metadata.Dependencies
	if sum, err := resolver.HashReq(req); err != nil || sum != lock.Digest {
		return errors.New("Chart.lock is out of sync with Chart.yaml")
	}

	// Now we need to fetch every package here into charts/
	if err := m.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	return nil
}

// Update updates a local charts directory.
//
// It first reads the Chart.yaml file, and then attempts to
// negotiate versions based on that. It will download the versions
// from remote chart repositories unless SkipUpdate is true.
func (m *Manager) Update() error {
	c, err := m.loadChartDir()
	if err != nil {
		return err
	}

	// If no dependencies are found, we consider this a successful
	// completion.
	req := c.Metadata.Dependencies
	if req == nil {
		return nil
	}

	// Hash dependencies
	// FIXME should this hash all of Chart.yaml
	hash, err := resolver.HashReq(req)
	if err != nil {
		return err
	}

	// Now we need to find out which version of a chart best satisfies the
	// dependencies in the Chart.yaml
	lock, err := m.resolve(req, hash)
	if err != nil {
		return err
	}

	// Now we need to fetch every package here into charts/
	if err := m.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	// If the lock file hasn't changed, don't write a new one.
	oldLock := c.Lock
	if oldLock != nil && oldLock.Digest == lock.Digest {
		return nil
	}

	// Finally, we need to write the lockfile.
	return writeLock(m.ChartPath, lock)
}

func (m *Manager) loadChartDir() (*chart.Chart, error) {
	if fi, err := os.Stat(m.ChartPath); err != nil {
		return nil, errors.Wrapf(err, "could not find %s", m.ChartPath)
	} else if !fi.IsDir() {
		return nil, errors.New("only unpacked charts can be updated")
	}
	return loader.LoadDir(m.ChartPath)
}

// resolve takes a list of dependencies and translates them into an exact version to download.
//
// This returns a lock file, which has all of the dependencies normalized to a specific version.
func (m *Manager) resolve(req []*chart.Dependency, hash string) (*chart.Lock, error) {
	res := resolver.New(m.ChartPath, m.Client)
	return res.Resolve(req, hash)
}

// downloadAll takes a list of dependencies and downloads them into charts/
//
// It will delete versions of the chart that exist on disk and might cause
// a conflict.
func (m *Manager) downloadAll(deps []*chart.Dependency) error {
	destPath := filepath.Join(m.ChartPath, "charts")
	tmpPath := filepath.Join(m.ChartPath, "tmpcharts")

	// Create 'charts' directory if it doesn't already exist.
	if fi, err := os.Stat(destPath); err != nil {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return err
		}
	} else if !fi.IsDir() {
		return errors.Errorf("%q is not a directory", destPath)
	}

	if err := os.Rename(destPath, tmpPath); err != nil {
		return errors.Wrap(err, "unable to move current charts to tmp dir")
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	fmt.Fprintf(m.Out, "Saving %d charts\n", len(deps))
	var saveError error
	for _, dep := range deps {
		if strings.HasPrefix(dep.Name, "file://") {
			if m.Debug {
				fmt.Fprintf(m.Out, "Archiving %s\n", dep.Name)
			}
			ver, err := tarFromLocalDir(m.ChartPath, dep.Name, dep.Version)
			if err != nil {
				saveError = err
				break
			}
			dep.Version = ver
			continue
		}

		fmt.Fprintf(m.Out, "Downloading %s\n", dep.Name)

		ref, err := repo.ParseRepoNameTag(dep.Repository, dep.Name, dep.Version)
		if err != nil {
			saveError = errors.Wrapf(err, "could not parse dependency %q", dep.Name)
			break
		}

		if err := m.Client.PullChart(ref); err != nil {
			saveError = errors.Wrapf(err, "could not download %q", ref.String())
			break
		}

		ch, err := m.Client.LoadChart(ref)
		if err != nil {
			saveError = errors.Wrapf(err, "could not download %q", ref.String())
			break
		}

		if _, err := chartutil.Save(ch, destPath); err != nil {
			saveError = errors.Wrapf(err, "could not download %s", ref.String())
			break
		}
	}

	if saveError == nil {
		fmt.Fprintln(m.Out, "Deleting outdated charts")
		for _, dep := range deps {
			if err := m.safeDeleteDep(dep.Name, tmpPath); err != nil {
				return err
			}
		}
		if err := move(tmpPath, destPath); err != nil {
			return err
		}
		if err := os.RemoveAll(tmpPath); err != nil {
			return errors.Wrapf(err, "failed to remove %v", tmpPath)
		}
	} else {
		fmt.Fprintln(m.Out, "Save error occurred: ", saveError)
		fmt.Fprintln(m.Out, "Deleting newly downloaded charts, restoring pre-update state")
		for _, dep := range deps {
			if err := m.safeDeleteDep(dep.Name, destPath); err != nil {
				return err
			}
		}
		if err := os.RemoveAll(destPath); err != nil {
			return errors.Wrapf(err, "failed to remove %v", destPath)
		}
		if err := os.Rename(tmpPath, destPath); err != nil {
			return errors.Wrap(err, "unable to move current charts to tmp dir")
		}
		return saveError
	}
	return nil
}

// safeDeleteDep deletes any versions of the given dependency in the given directory.
//
// It does this by first matching the file name to an expected pattern, then loading
// the file to verify that it is a chart with the same name as the given name.
//
// Because it requires tar file introspection, it is more intensive than a basic delete.
//
// This will only return errors that should stop processing entirely. Other errors
// will emit log messages or be ignored.
func (m *Manager) safeDeleteDep(name, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, name+"-*.tgz"))
	if err != nil {
		// Only for ErrBadPattern
		return err
	}
	for _, fname := range files {
		ch, err := loader.LoadFile(fname)
		if err != nil {
			fmt.Fprintf(m.Out, "Could not verify %s for deletion: %s (Skipping)", fname, err)
			continue
		}
		if ch.Name() != name {
			// This is not the file you are looking for.
			continue
		}
		if err := os.Remove(fname); err != nil {
			fmt.Fprintf(m.Out, "Could not delete %s: %s (Skipping)", fname, err)
			continue
		}
	}
	return nil
}

func versionEquals(v1, v2 string) bool {
	sv1, err := semver.NewVersion(v1)
	if err != nil {
		// Fallback to string comparison.
		return v1 == v2
	}
	sv2, err := semver.NewVersion(v2)
	if err != nil {
		return false
	}
	return sv1.Equal(sv2)
}

// writeLock writes a lockfile to disk
func writeLock(chartpath string, lock *chart.Lock) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	dest := filepath.Join(chartpath, "Chart.lock")
	return ioutil.WriteFile(dest, data, 0644)
}

// archive a dep chart from local directory and save it into charts/
func tarFromLocalDir(chartpath, name, version string) (string, error) {
	destPath := filepath.Join(chartpath, "charts")

	if !strings.HasPrefix(name, "file://") {
		return "", errors.Errorf("wrong format: chart %s", name)
	}

	origPath, err := resolver.GetLocalPath(name, chartpath)
	if err != nil {
		return "", err
	}

	ch, err := loader.LoadDir(origPath)
	if err != nil {
		return "", err
	}

	constraint, err := semver.NewConstraint(version)
	if err != nil {
		return "", errors.Wrapf(err, "dependency %s has an invalid version/constraint format", name)
	}

	v, err := semver.NewVersion(ch.Metadata.Version)
	if err != nil {
		return "", err
	}

	if constraint.Check(v) {
		_, err = chartutil.Save(ch, destPath)
		return ch.Metadata.Version, err
	}

	return "", errors.Errorf("can't get a valid version for dependency %s", name)
}

// move files from tmppath to destpath
func move(tmpPath, destPath string) error {
	files, _ := ioutil.ReadDir(tmpPath)
	for _, file := range files {
		filename := file.Name()
		tmpfile := filepath.Join(tmpPath, filename)
		destfile := filepath.Join(destPath, filename)
		if err := os.Rename(tmpfile, destfile); err != nil {
			return errors.Wrap(err, "unable to move local charts to charts dir")
		}
	}
	return nil
}
