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
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/internal/resolver"
	"helm.sh/helm/v3/internal/third_party/dep/fs"
	"helm.sh/helm/v3/internal/urlutil"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

// Manager handles the lifecycle of fetching, resolving, and storing dependencies.
type Manager struct {
	// Out is used to print warnings and notifications.
	Out io.Writer
	// ChartPath is the path to the unpacked base chart upon which this operates.
	ChartPath string
	// Verification indicates whether the chart should be verified.
	Verify VerificationStrategy
	// Debug is the global "--debug" flag
	Debug bool
	// Keyring is the key ring file.
	Keyring string
	// SkipUpdate indicates that the repository should not be updated first.
	SkipUpdate bool
	// Getter collection for the operation
	Getters          []getter.Provider
	RepositoryConfig string
	RepositoryCache  string
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

	// Check that all of the repos we're dependent on actually exist.
	req := c.Metadata.Dependencies

	// If using apiVersion v1, calculate the hash before resolve repo names
	// because resolveRepoNames will change req if req uses repo alias
	// and Helm 2 calculate the digest from the original req
	// Fix for: https://github.com/helm/helm/issues/7619
	var v2Sum string
	if c.Metadata.APIVersion == chart.APIVersionV1 {
		v2Sum, err = resolver.HashV2Req(req)
		if err != nil {
			return errors.New("the lock file (requirements.lock) is out of sync with the dependencies file (requirements.yaml). Please update the dependencies")
		}
	}

	if _, err := m.resolveRepoNames(req); err != nil {
		return err
	}

	if sum, err := resolver.HashReq(req, lock.Dependencies); err != nil || sum != lock.Digest {
		// If lock digest differs and chart is apiVersion v1, it maybe because the lock was built
		// with Helm 2 and therefore should be checked with Helm v2 hash
		// Fix for: https://github.com/helm/helm/issues/7233
		if c.Metadata.APIVersion == chart.APIVersionV1 {
			log.Println("warning: a valid Helm v3 hash was not found. Checking against Helm v2 hash...")
			if v2Sum != lock.Digest {
				return errors.New("the lock file (requirements.lock) is out of sync with the dependencies file (requirements.yaml). Please update the dependencies")
			}
		} else {
			return errors.New("the lock file (Chart.lock) is out of sync with the dependencies file (Chart.yaml). Please update the dependencies")
		}
	}

	// Check that all of the repos we're dependent on actually exist.
	if err := m.hasAllRepos(lock.Dependencies); err != nil {
		return err
	}

	if !m.SkipUpdate {
		// For each repo in the file, update the cached copy of that repo
		if err := m.UpdateRepositories(); err != nil {
			return err
		}
	}

	// Now we need to fetch every package here into charts/
	return m.downloadAll(lock.Dependencies)
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

	// Check that all of the repos we're dependent on actually exist and
	// the repo index names.
	repoNames, err := m.resolveRepoNames(req)
	if err != nil {
		return err
	}

	// For each repo in the file, update the cached copy of that repo
	if !m.SkipUpdate {
		if err := m.UpdateRepositories(); err != nil {
			return err
		}
	}

	// Now we need to find out which version of a chart best satisfies the
	// dependencies in the Chart.yaml
	lock, err := m.resolve(req, repoNames)
	if err != nil {
		return err
	}

	// Now we need to fetch every package here into charts/
	if err := m.downloadAll(lock.Dependencies); err != nil {
		return err
	}

	// downloadAll might overwrite dependency version, recalculate lock digest
	newDigest, err := resolver.HashReq(req, lock.Dependencies)
	if err != nil {
		return err
	}
	lock.Digest = newDigest

	// If the lock file hasn't changed, don't write a new one.
	oldLock := c.Lock
	if oldLock != nil && oldLock.Digest == lock.Digest {
		return nil
	}

	// Finally, we need to write the lockfile.
	return writeLock(m.ChartPath, lock, c.Metadata.APIVersion == chart.APIVersionV1)
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
func (m *Manager) resolve(req []*chart.Dependency, repoNames map[string]string) (*chart.Lock, error) {
	res := resolver.New(m.ChartPath, m.RepositoryCache)
	return res.Resolve(req, repoNames)
}

// downloadAll takes a list of dependencies and downloads them into charts/
//
// It will delete versions of the chart that exist on disk and might cause
// a conflict.
func (m *Manager) downloadAll(deps []*chart.Dependency) error {
	repos, err := m.loadChartRepositories()
	if err != nil {
		return err
	}

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

	if err := fs.RenameWithFallback(destPath, tmpPath); err != nil {
		return errors.Wrap(err, "unable to move current charts to tmp dir")
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	fmt.Fprintf(m.Out, "Saving %d charts\n", len(deps))
	var saveError error
	churls := make(map[string]struct{})
	for _, dep := range deps {
		// No repository means the chart is in charts directory
		if dep.Repository == "" {
			fmt.Fprintf(m.Out, "Dependency %s did not declare a repository. Assuming it exists in the charts directory\n", dep.Name)
			chartPath := filepath.Join(tmpPath, dep.Name)
			ch, err := loader.LoadDir(chartPath)
			if err != nil {
				return fmt.Errorf("Unable to load chart: %v", err)
			}

			constraint, err := semver.NewConstraint(dep.Version)
			if err != nil {
				return fmt.Errorf("Dependency %s has an invalid version/constraint format: %s", dep.Name, err)
			}

			v, err := semver.NewVersion(ch.Metadata.Version)
			if err != nil {
				return fmt.Errorf("Invalid version %s for dependency %s: %s", dep.Version, dep.Name, err)
			}

			if !constraint.Check(v) {
				saveError = fmt.Errorf("Dependency %s at version %s does not satisfy the constraint %s", dep.Name, ch.Metadata.Version, dep.Version)
				break
			}
			continue
		}
		if strings.HasPrefix(dep.Repository, "file://") {
			if m.Debug {
				fmt.Fprintf(m.Out, "Archiving %s from repo %s\n", dep.Name, dep.Repository)
			}
			ver, err := tarFromLocalDir(m.ChartPath, dep.Name, dep.Repository, dep.Version)
			if err != nil {
				saveError = err
				break
			}
			dep.Version = ver
			continue
		}

		// Any failure to resolve/download a chart should fail:
		// https://github.com/helm/helm/issues/1439
		churl, username, password, err := m.findChartURL(dep.Name, dep.Version, dep.Repository, repos)
		if err != nil {
			saveError = errors.Wrapf(err, "could not find %s", churl)
			break
		}

		if _, ok := churls[churl]; ok {
			fmt.Fprintf(m.Out, "Already downloaded %s from repo %s\n", dep.Name, dep.Repository)
			continue
		}

		fmt.Fprintf(m.Out, "Downloading %s from repo %s\n", dep.Name, dep.Repository)

		dl := ChartDownloader{
			Out:              m.Out,
			Verify:           m.Verify,
			Keyring:          m.Keyring,
			RepositoryConfig: m.RepositoryConfig,
			RepositoryCache:  m.RepositoryCache,
			Getters:          m.Getters,
			Options: []getter.Option{
				getter.WithBasicAuth(username, password),
			},
		}

		if _, _, err := dl.DownloadTo(churl, "", destPath); err != nil {
			saveError = errors.Wrapf(err, "could not download %s", churl)
			break
		}

		churls[churl] = struct{}{}
	}

	if saveError == nil {
		fmt.Fprintln(m.Out, "Deleting outdated charts")
		for _, dep := range deps {
			// Chart from local charts directory stays in place
			if dep.Repository != "" {
				if err := m.safeDeleteDep(dep.Name, tmpPath); err != nil {
					return err
				}
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
		if err := fs.RenameWithFallback(tmpPath, destPath); err != nil {
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

// hasAllRepos ensures that all of the referenced deps are in the local repo cache.
func (m *Manager) hasAllRepos(deps []*chart.Dependency) error {
	rf, err := loadRepoConfig(m.RepositoryConfig)
	if err != nil {
		return err
	}
	repos := rf.Repositories

	// Verify that all repositories referenced in the deps are actually known
	// by Helm.
	missing := []string{}
Loop:
	for _, dd := range deps {
		// If repo is from local path, continue
		if strings.HasPrefix(dd.Repository, "file://") {
			continue
		}

		if dd.Repository == "" {
			continue
		}
		for _, repo := range repos {
			if urlutil.Equal(repo.URL, strings.TrimSuffix(dd.Repository, "/")) {
				continue Loop
			}
		}
		missing = append(missing, dd.Repository)
	}
	if len(missing) > 0 {
		return errors.Errorf("no repository definition for %s. Please add the missing repos via 'helm repo add'", strings.Join(missing, ", "))
	}
	return nil
}

// resolveRepoNames returns the repo names of the referenced deps which can be used to fetch the cached index file
// and replaces aliased repository URLs into resolved URLs in dependencies.
func (m *Manager) resolveRepoNames(deps []*chart.Dependency) (map[string]string, error) {
	rf, err := loadRepoConfig(m.RepositoryConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	repos := rf.Repositories

	reposMap := make(map[string]string)

	// Verify that all repositories referenced in the deps are actually known
	// by Helm.
	missing := []string{}
	for _, dd := range deps {
		// Don't map the repository, we don't need to download chart from charts directory
		if dd.Repository == "" {
			continue
		}
		// if dep chart is from local path, verify the path is valid
		if strings.HasPrefix(dd.Repository, "file://") {
			if _, err := resolver.GetLocalPath(dd.Repository, m.ChartPath); err != nil {
				return nil, err
			}

			if m.Debug {
				fmt.Fprintf(m.Out, "Repository from local path: %s\n", dd.Repository)
			}
			reposMap[dd.Name] = dd.Repository
			continue
		}

		found := false

		for _, repo := range repos {
			if (strings.HasPrefix(dd.Repository, "@") && strings.TrimPrefix(dd.Repository, "@") == repo.Name) ||
				(strings.HasPrefix(dd.Repository, "alias:") && strings.TrimPrefix(dd.Repository, "alias:") == repo.Name) {
				found = true
				dd.Repository = repo.URL
				reposMap[dd.Name] = repo.Name
				break
			} else if urlutil.Equal(repo.URL, dd.Repository) {
				found = true
				reposMap[dd.Name] = repo.Name
				break
			}
		}
		if !found {
			repository := dd.Repository
			// Add if URL
			_, err := url.ParseRequestURI(repository)
			if err == nil {
				reposMap[repository] = repository
				continue
			}
			missing = append(missing, repository)
		}
	}
	if len(missing) > 0 {
		errorMessage := fmt.Sprintf("no repository definition for %s. Please add them via 'helm repo add'", strings.Join(missing, ", "))
		// It is common for people to try to enter "stable" as a repository instead of the actual URL.
		// For this case, let's give them a suggestion.
		containsNonURL := false
		for _, repo := range missing {
			if !strings.Contains(repo, "//") && !strings.HasPrefix(repo, "@") && !strings.HasPrefix(repo, "alias:") {
				containsNonURL = true
			}
		}
		if containsNonURL {
			errorMessage += `
Note that repositories must be URLs or aliases. For example, to refer to the "example"
repository, use "https://charts.example.com/" or "@example" instead of
"example". Don't forget to add the repo, too ('helm repo add').`
		}
		return nil, errors.New(errorMessage)
	}
	return reposMap, nil
}

// UpdateRepositories updates all of the local repos to the latest.
func (m *Manager) UpdateRepositories() error {
	rf, err := loadRepoConfig(m.RepositoryConfig)
	if err != nil {
		return err
	}
	repos := rf.Repositories
	if len(repos) > 0 {
		// This prints warnings straight to out.
		if err := m.parallelRepoUpdate(repos); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) parallelRepoUpdate(repos []*repo.Entry) error {
	fmt.Fprintln(m.Out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for _, c := range repos {
		r, err := repo.NewChartRepository(c, m.Getters)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func(r *repo.ChartRepository) {
			if _, err := r.DownloadIndexFile(); err != nil {
				fmt.Fprintf(m.Out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", r.Config.Name, r.Config.URL, err)
			} else {
				fmt.Fprintf(m.Out, "...Successfully got an update from the %q chart repository\n", r.Config.Name)
			}
			wg.Done()
		}(r)
	}
	wg.Wait()
	fmt.Fprintln(m.Out, "Update Complete. ⎈Happy Helming!⎈")
	return nil
}

// findChartURL searches the cache of repo data for a chart that has the name and the repoURL specified.
//
// 'name' is the name of the chart. Version is an exact semver, or an empty string. If empty, the
// newest version will be returned.
//
// repoURL is the repository to search
//
// If it finds a URL that is "relative", it will prepend the repoURL.
func (m *Manager) findChartURL(name, version, repoURL string, repos map[string]*repo.ChartRepository) (url, username, password string, err error) {
	for _, cr := range repos {
		if urlutil.Equal(repoURL, cr.Config.URL) {
			var entry repo.ChartVersions
			entry, err = findEntryByName(name, cr)
			if err != nil {
				return
			}
			var ve *repo.ChartVersion
			ve, err = findVersionedEntry(version, entry)
			if err != nil {
				return
			}
			url, err = normalizeURL(repoURL, ve.URLs[0])
			if err != nil {
				return
			}
			username = cr.Config.Username
			password = cr.Config.Password
			return
		}
	}
	url, err = repo.FindChartInRepoURL(repoURL, name, version, "", "", "", m.Getters)
	if err == nil {
		return
	}
	err = errors.Errorf("chart %s not found in %s", name, repoURL)
	return
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
		return urlOrPath, errors.Wrap(err, "base URL failed to parse")
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

	// Load repositories.yaml file
	rf, err := loadRepoConfig(m.RepositoryConfig)
	if err != nil {
		return indices, errors.Wrapf(err, "failed to load %s", m.RepositoryConfig)
	}

	for _, re := range rf.Repositories {
		lname := re.Name
		idxFile := filepath.Join(m.RepositoryCache, helmpath.CacheIndexFile(lname))
		index, err := repo.LoadIndexFile(idxFile)
		if err != nil {
			return indices, err
		}

		// TODO: use constructor
		cr := &repo.ChartRepository{
			Config:    re,
			IndexFile: index,
		}
		indices[lname] = cr
	}
	return indices, nil
}

// writeLock writes a lockfile to disk
func writeLock(chartpath string, lock *chart.Lock, legacyLockfile bool) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	lockfileName := "Chart.lock"
	if legacyLockfile {
		lockfileName = "requirements.lock"
	}
	dest := filepath.Join(chartpath, lockfileName)
	return ioutil.WriteFile(dest, data, 0644)
}

// archive a dep chart from local directory and save it into charts/
func tarFromLocalDir(chartpath, name, repo, version string) (string, error) {
	destPath := filepath.Join(chartpath, "charts")

	if !strings.HasPrefix(repo, "file://") {
		return "", errors.Errorf("wrong format: chart %s repository %s", name, repo)
	}

	origPath, err := resolver.GetLocalPath(repo, chartpath)
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
		if err := fs.RenameWithFallback(tmpfile, destfile); err != nil {
			return errors.Wrap(err, "unable to move local charts to charts dir")
		}
	}
	return nil
}
