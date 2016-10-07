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

package downloader

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/cmd/helm/resolver"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/repo"
)

// Manager handles the lifecycle of fetching, resolving, and storing dependencies.
type Manager struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// ChartPath is the path to the unpacked base chart upon which this operates.
	ChartPath string
	// HelmHome is the $HELM_HOME directory
	HelmHome helmpath.Home
	// Verification indicates whether the chart should be verified.
	Verify VerificationStrategy
	// Keyring is the key ring file.
	Keyring string
}

// Build rebuilds a local charts directory from a lockfile.
//
// If the lockfile is not present, this will run a Manager.Update()
func (m *Manager) Build() error {
	c, err := m.loadChartDir()
	if err != nil {
		return err
	}

	// If a lock file is found, run a build from that. Otherwise, just do
	// an update.
	lock, err := chartutil.LoadRequirementsLock(c)
	if err != nil {
		return m.Update()
	}

	// A lock must accompany a requirements.yaml file.
	req, err := chartutil.LoadRequirements(c)
	if err != nil {
		return fmt.Errorf("requirements.yaml cannot be opened: %s", err)
	}
	if sum, err := resolver.HashReq(req); err != nil || sum != lock.Digest {
		return fmt.Errorf("requirements.lock is out of sync with requirements.yaml")
	}

	// Check that all of the repos we're dependent on actually exist.
	if err := m.hasAllRepos(lock.Dependencies); err != nil {
		return err
	}

	// For each repo in the file, update the cached copy of that repo
	if err := m.UpdateRepositories(); err != nil {
		return err
	}

	// Now we need to fetch every package here into charts/
	if err := m.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	return nil
}

// Update updates a local charts directory.
//
// It first reads the requirements.yaml file, and then attempts to
// negotiate versions based on that. It will download the versions
// from remote chart repositories.
func (m *Manager) Update() error {
	c, err := m.loadChartDir()
	if err != nil {
		return err
	}

	// If no requirements file is found, we consider this a successful
	// completion.
	req, err := chartutil.LoadRequirements(c)
	if err != nil {
		if err == chartutil.ErrRequirementsNotFound {
			fmt.Fprintf(m.Out, "No requirements found in %s/charts.\n", m.ChartPath)
			return nil
		}
		return err
	}

	// Check that all of the repos we're dependent on actually exist.
	if err := m.hasAllRepos(req.Dependencies); err != nil {
		return err
	}

	// For each repo in the file, update the cached copy of that repo
	if err := m.UpdateRepositories(); err != nil {
		return err
	}

	// Now we need to find out which version of a chart best satisfies the
	// requirements the requirements.yaml
	lock, err := m.resolve(req)
	if err != nil {
		return err
	}

	// Now we need to fetch every package here into charts/
	if err := m.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	// Finally, we need to write the lockfile.
	return writeLock(m.ChartPath, lock)
}

func (m *Manager) loadChartDir() (*chart.Chart, error) {
	if fi, err := os.Stat(m.ChartPath); err != nil {
		return nil, fmt.Errorf("could not find %s: %s", m.ChartPath, err)
	} else if !fi.IsDir() {
		return nil, errors.New("only unpacked charts can be updated")
	}
	return chartutil.LoadDir(m.ChartPath)
}

// resolve takes a list of requirements and translates them into an exact version to download.
//
// This returns a lock file, which has all of the requirements normalized to a specific version.
func (m *Manager) resolve(req *chartutil.Requirements) (*chartutil.RequirementsLock, error) {
	res := resolver.New(m.ChartPath, m.HelmHome)
	return res.Resolve(req)
}

// downloadAll takes a list of dependencies and downloads them into charts/
func (m *Manager) downloadAll(deps []*chartutil.Dependency) error {
	repos, err := m.loadChartRepositories()
	if err != nil {
		return err
	}

	dl := ChartDownloader{
		Out:      m.Out,
		Verify:   m.Verify,
		Keyring:  m.Keyring,
		HelmHome: m.HelmHome,
	}

	fmt.Fprintf(m.Out, "Saving %d charts\n", len(deps))
	for _, dep := range deps {
		fmt.Fprintf(m.Out, "Downloading %s from repo %s\n", dep.Name, dep.Repository)

		churl, err := findChartURL(dep.Name, dep.Version, dep.Repository, repos)
		if err != nil {
			fmt.Fprintf(m.Out, "WARNING: %s (skipped)", err)
			continue
		}

		dest := filepath.Join(m.ChartPath, "charts")
		if _, _, err := dl.DownloadTo(churl, "", dest); err != nil {
			fmt.Fprintf(m.Out, "WARNING: Could not download %s: %s (skipped)", churl, err)
			continue
		}
	}
	return nil
}

// hasAllRepos ensures that all of the referenced deps are in the local repo cache.
func (m *Manager) hasAllRepos(deps []*chartutil.Dependency) error {
	rf, err := repo.LoadRepositoriesFile(m.HelmHome.RepositoryFile())
	if err != nil {
		return err
	}
	repos := rf.Repositories
	// Verify that all repositories referenced in the deps are actually known
	// by Helm.
	missing := []string{}
	for _, dd := range deps {
		found := false
		if dd.Repository == "" {
			found = true
		} else {
			for _, repo := range repos {
				if urlsAreEqual(repo.URL, dd.Repository) {
					found = true
				}
			}
		}
		if !found {
			missing = append(missing, dd.Repository)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("no repository definition for %s. Try 'helm repo add'", strings.Join(missing, ", "))
	}
	return nil
}

// UpdateRepositories updates all of the local repos to the latest.
func (m *Manager) UpdateRepositories() error {
	rf, err := repo.LoadRepositoriesFile(m.HelmHome.RepositoryFile())
	if err != nil {
		return err
	}
	repos := rf.Repositories
	if len(repos) > 0 {
		// This prints warnings straight to out.
		m.parallelRepoUpdate(repos)
	}
	return nil
}

func (m *Manager) parallelRepoUpdate(repos []*repo.Entry) {
	out := m.Out
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(n, u string) {
			if err := repo.DownloadIndexFile(n, u, m.HelmHome.CacheIndex(n)); err != nil {
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", n, u, err)
			} else {
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", n)
			}
			wg.Done()
		}(re.Name, re.URL)
	}
	wg.Wait()
	fmt.Fprintln(out, "Update Complete. ⎈Happy Helming!⎈")
}

// urlsAreEqual normalizes two URLs and then compares for equality.
func urlsAreEqual(a, b string) bool {
	au, err := url.Parse(a)
	if err != nil {
		// If urls are paths, return true only if they are an exact match
		return a == b
	}
	bu, err := url.Parse(b)
	if err != nil {
		return false
	}
	return au.String() == bu.String()
}

// findChartURL searches the cache of repo data for a chart that has the name and the repoURL specified.
//
// 'name' is the name of the chart. Version is an exact semver, or an empty string. If empty, the
// newest version will be returned.
//
// repoURL is the repository to search
//
// If it finds a URL that is "relative", it will prepend the repoURL.
func findChartURL(name, version, repoURL string, repos map[string]*repo.ChartRepository) (string, error) {
	for _, cr := range repos {
		if urlsAreEqual(repoURL, cr.URL) {
			entry, err := findEntryByName(name, cr)
			if err != nil {
				return "", err
			}
			ve, err := findVersionedEntry(version, entry)
			if err != nil {
				return "", err
			}

			return normalizeURL(repoURL, ve.URLs[0])
		}
	}
	return "", fmt.Errorf("chart %s not found in %s", name, repoURL)
}

// findEntryByName finds an entry in the chart repository whose name matches the given name.
//
// It returns the ChartVersions for that entry.
func findEntryByName(name string, cr *repo.ChartRepository) (repo.ChartVersions, error) {
	for ename, entry := range cr.IndexFile.Entries {
		if ename == name {
			return entry, nil
		}
	}
	return nil, errors.New("entry not found")
}

// findVersionedEntry takes a ChartVersions list and returns a single chart version that satisfies the version constraints.
//
// If version is empty, the first chart found is returned.
func findVersionedEntry(version string, vers repo.ChartVersions) (*repo.ChartVersion, error) {
	for _, verEntry := range vers {
		if len(verEntry.URLs) == 0 {
			// Not a legit entry.
			continue
		}

		if version == "" || versionEquals(version, verEntry.Version) {
			return verEntry, nil
		}

		return verEntry, nil
	}
	return nil, errors.New("no matching version")
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

func normalizeURL(baseURL, urlOrPath string) (string, error) {
	u, err := url.Parse(urlOrPath)
	if err != nil {
		return urlOrPath, err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	u2, err := url.Parse(baseURL)
	if err != nil {
		return urlOrPath, fmt.Errorf("Base URL failed to parse: %s", err)
	}

	u2.Path = path.Join(u2.Path, urlOrPath)
	return u2.String(), nil
}

// loadChartRepositories reads the repositories.yaml, and then builds a map of
// ChartRepositories.
//
// The key is the local name (which is only present in the repositories.yaml).
func (m *Manager) loadChartRepositories() (map[string]*repo.ChartRepository, error) {
	indices := map[string]*repo.ChartRepository{}
	repoyaml := m.HelmHome.RepositoryFile()

	// Load repositories.yaml file
	rf, err := repo.LoadRepositoriesFile(repoyaml)
	if err != nil {
		return indices, fmt.Errorf("failed to load %s: %s", repoyaml, err)
	}

	for _, re := range rf.Repositories {
		lname := re.Name
		cacheindex := m.HelmHome.CacheIndex(lname)
		index, err := repo.LoadIndexFile(cacheindex)
		if err != nil {
			return indices, err
		}

		cr := &repo.ChartRepository{
			URL:       re.URL,
			IndexFile: index,
		}
		indices[lname] = cr
	}
	return indices, nil
}

// writeLock writes a lockfile to disk
func writeLock(chartpath string, lock *chartutil.RequirementsLock) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	dest := filepath.Join(chartpath, "requirements.lock")
	return ioutil.WriteFile(dest, data, 0644)
}
