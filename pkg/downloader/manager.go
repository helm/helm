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
	"crypto"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
)

// ErrRepoNotFound indicates that chart repositories can't be found in local repo cache.
// The value of Repos is missing repos.
type ErrRepoNotFound struct {
	Repos []string
}

// Error implements the error interface.
func (e ErrRepoNotFound) Error() string {
	return fmt.Sprintf("no repository definition for %s", strings.Join(e.Repos, ", "))
}

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
	RegistryClient   *registry.Client
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

	// Get the names of the repositories the dependencies need that Helm is
	// configured to know about.
	repoNames, err := m.resolveRepoNames(req)
	if err != nil {
		return err
	}

	// For the repositories Helm is not configured to know about, ensure Helm
	// has some information about them and, when possible, the index files
	// locally.
	// TODO(mattfarina): Repositories should be explicitly added by end users
	// rather than automatic. In Helm v4 require users to add repositories. They
	// should have to add them in order to make sure they are aware of the
	// repositories and opt-in to any locations, for security.
	repoNames, err = m.ensureMissingRepos(repoNames, req)
	if err != nil {
		return err
	}

	// For each of the repositories Helm is configured to know about, update
	// the index information locally.
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
	res := resolver.New(m.ChartPath, m.RepositoryCache, m.RegistryClient)
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
	tmpPath := filepath.Join(m.ChartPath, fmt.Sprintf("tmpcharts-%d", os.Getpid()))

	// Check if 'charts' directory is not actually a directory. If it does not exist, create it.
	if fi, err := os.Stat(destPath); err == nil {
		if !fi.IsDir() {
			return errors.Errorf("%q is not a directory", destPath)
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unable to retrieve file info for '%s': %v", destPath, err)
	}

	// Prepare tmpPath
	if err := os.MkdirAll(tmpPath, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpPath)

	fmt.Fprintf(m.Out, "Saving %d charts\n", len(deps))
	var saveError error
	churls := make(map[string]struct{})
	for _, dep := range deps {
		// No repository means the chart is in charts directory
		if dep.Repository == "" {
			fmt.Fprintf(m.Out, "Dependency %s did not declare a repository. Assuming it exists in the charts directory\n", dep.Name)
			// NOTE: we are only validating the local dependency conforms to the constraints. No copying to tmpPath is necessary.
			chartPath := filepath.Join(destPath, dep.Name)
			ch, err := loader.LoadDir(chartPath)
			if err != nil {
				return fmt.Errorf("unable to load chart '%s': %v", chartPath, err)
			}

			constraint, err := semver.NewConstraint(dep.Version)
			if err != nil {
				return fmt.Errorf("dependency %s has an invalid version/constraint format: %s", dep.Name, err)
			}

			v, err := semver.NewVersion(ch.Metadata.Version)
			if err != nil {
				return fmt.Errorf("invalid version %s for dependency %s: %s", dep.Version, dep.Name, err)
			}

			if !constraint.Check(v) {
				saveError = fmt.Errorf("dependency %s at version %s does not satisfy the constraint %s", dep.Name, ch.Metadata.Version, dep.Version)
				break
			}
			continue
		}
		if strings.HasPrefix(dep.Repository, "file://") {
			if m.Debug {
				fmt.Fprintf(m.Out, "Archiving %s from repo %s\n", dep.Name, dep.Repository)
			}
			ver, err := tarFromLocalDir(m.ChartPath, dep.Name, dep.Repository, dep.Version, tmpPath)
			if err != nil {
				saveError = err
				break
			}
			dep.Version = ver
			continue
		}

		// Any failure to resolve/download a chart should fail:
		// https://github.com/helm/helm/issues/1439
		churl, username, password, insecureskiptlsverify, passcredentialsall, caFile, certFile, keyFile, err := m.findChartURL(dep.Name, dep.Version, dep.Repository, repos)
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
			RegistryClient:   m.RegistryClient,
			Getters:          m.Getters,
			Options: []getter.Option{
				getter.WithBasicAuth(username, password),
				getter.WithPassCredentialsAll(passcredentialsall),
				getter.WithInsecureSkipVerifyTLS(insecureskiptlsverify),
				getter.WithTLSClientConfig(certFile, keyFile, caFile),
			},
		}

		version := ""
		if registry.IsOCI(churl) {
			churl, version, err = parseOCIRef(churl)
			if err != nil {
				return errors.Wrapf(err, "could not parse OCI reference")
			}
			dl.Options = append(dl.Options,
				getter.WithRegistryClient(m.RegistryClient),
				getter.WithTagName(version))
		}

		if _, _, err = dl.DownloadTo(churl, version, tmpPath); err != nil {
			saveError = errors.Wrapf(err, "could not download %s", churl)
			break
		}

		churls[churl] = struct{}{}
	}

	// TODO: this should probably be refactored to be a []error, so we can capture and provide more information rather than "last error wins".
	if saveError == nil {
		// now we can move all downloaded charts to destPath and delete outdated dependencies
		if err := m.safeMoveDeps(deps, tmpPath, destPath); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(m.Out, "Save error occurred: ", saveError)
		return saveError
	}
	return nil
}

func parseOCIRef(chartRef string) (string, string, error) {
	refTagRegexp := regexp.MustCompile(`^(oci://[^:]+(:[0-9]{1,5})?[^:]+):(.*)$`)
	caps := refTagRegexp.FindStringSubmatch(chartRef)
	if len(caps) != 4 {
		return "", "", errors.Errorf("improperly formatted oci chart reference: %s", chartRef)
	}
	chartRef = caps[1]
	tag := caps[3]

	return chartRef, tag, nil
}

// safeMoveDep moves all dependencies in the source and moves them into dest.
//
// It does this by first matching the file name to an expected pattern, then loading
// the file to verify that it is a chart.
//
// Any charts in dest that do not exist in source are removed (barring local dependencies)
//
// Because it requires tar file introspection, it is more intensive than a basic move.
//
// This will only return errors that should stop processing entirely. Other errors
// will emit log messages or be ignored.
func (m *Manager) safeMoveDeps(deps []*chart.Dependency, source, dest string) error {
	existsInSourceDirectory := map[string]bool{}
	isLocalDependency := map[string]bool{}
	sourceFiles, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	// attempt to read destFiles; fail fast if we can't
	destFiles, err := os.ReadDir(dest)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		if dep.Repository == "" {
			isLocalDependency[dep.Name] = true
		}
	}

	for _, file := range sourceFiles {
		if file.IsDir() {
			continue
		}
		filename := file.Name()
		sourcefile := filepath.Join(source, filename)
		destfile := filepath.Join(dest, filename)
		existsInSourceDirectory[filename] = true
		if _, err := loader.LoadFile(sourcefile); err != nil {
			fmt.Fprintf(m.Out, "Could not verify %s for moving: %s (Skipping)", sourcefile, err)
			continue
		}
		// NOTE: no need to delete the dest; os.Rename replaces it.
		if err := fs.RenameWithFallback(sourcefile, destfile); err != nil {
			fmt.Fprintf(m.Out, "Unable to move %s to charts dir %s (Skipping)", sourcefile, err)
			continue
		}
	}

	fmt.Fprintln(m.Out, "Deleting outdated charts")
	// find all files that exist in dest that do not exist in source; delete them (outdated dependencies)
	for _, file := range destFiles {
		if !file.IsDir() && !existsInSourceDirectory[file.Name()] {
			fname := filepath.Join(dest, file.Name())
			ch, err := loader.LoadFile(fname)
			if err != nil {
				fmt.Fprintf(m.Out, "Could not verify %s for deletion: %s (Skipping)\n", fname, err)
				continue
			}
			// local dependency - skip
			if isLocalDependency[ch.Name()] {
				continue
			}
			if err := os.Remove(fname); err != nil {
				fmt.Fprintf(m.Out, "Could not delete %s: %s (Skipping)", fname, err)
				continue
			}
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
		// If repo is from local path or OCI, continue
		if strings.HasPrefix(dd.Repository, "file://") || registry.IsOCI(dd.Repository) {
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
		return ErrRepoNotFound{missing}
	}
	return nil
}

// ensureMissingRepos attempts to ensure the repository information for repos
// not managed by Helm is present. This takes in the repoNames Helm is configured
// to work with along with the chart dependencies. It will find the deps not
// in a known repo and attempt to ensure the data is present for steps like
// version resolution.
func (m *Manager) ensureMissingRepos(repoNames map[string]string, deps []*chart.Dependency) (map[string]string, error) {

	var ru []*repo.Entry

	for _, dd := range deps {

		// If the chart is in the local charts directory no repository needs
		// to be specified.
		if dd.Repository == "" {
			continue
		}

		// When the repoName for a dependency is known we can skip ensuring
		if _, ok := repoNames[dd.Name]; ok {
			continue
		}

		// The generated repository name, which will result in an index being
		// locally cached, has a name pattern of "helm-manager-" followed by a
		// sha256 of the repo name. This assumes end users will never create
		// repositories with these names pointing to other repositories. Using
		// this method of naming allows the existing repository pulling and
		// resolution code to do most of the work.
		rn, err := key(dd.Repository)
		if err != nil {
			return repoNames, err
		}
		rn = managerKeyPrefix + rn

		repoNames[dd.Name] = rn

		// Assuming the repository is generally available. For Helm managed
		// access controls the repository needs to be added through the user
		// managed system. This path will work for public charts, like those
		// supplied by Bitnami, but not for protected charts, like corp ones
		// behind a username and pass.
		ri := &repo.Entry{
			Name: rn,
			URL:  dd.Repository,
		}
		ru = append(ru, ri)
	}

	// Calls to UpdateRepositories (a public function) will only update
	// repositories configured by the user. Here we update repos found in
	// the dependencies that are not known to the user if update skipping
	// is not configured.
	if !m.SkipUpdate && len(ru) > 0 {
		fmt.Fprintln(m.Out, "Getting updates for unmanaged Helm repositories...")
		if err := m.parallelRepoUpdate(ru); err != nil {
			return repoNames, err
		}
	}

	return repoNames, nil
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

		if registry.IsOCI(dd.Repository) {
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
		fmt.Fprintln(m.Out, "Hang tight while we grab the latest from your chart repositories...")
		// This prints warnings straight to out.
		if err := m.parallelRepoUpdate(repos); err != nil {
			return err
		}
		fmt.Fprintln(m.Out, "Update Complete. ⎈Happy Helming!⎈")
	}
	return nil
}

func (m *Manager) parallelRepoUpdate(repos []*repo.Entry) error {

	var wg sync.WaitGroup
	for _, c := range repos {
		r, err := repo.NewChartRepository(c, m.Getters)
		if err != nil {
			return err
		}
		r.CachePath = m.RepositoryCache
		wg.Add(1)
		go func(r *repo.ChartRepository) {
			if _, err := r.DownloadIndexFile(); err != nil {
				// For those dependencies that are not known to helm and using a
				// generated key name we display the repo url.
				if strings.HasPrefix(r.Config.Name, managerKeyPrefix) {
					fmt.Fprintf(m.Out, "...Unable to get an update from the %q chart repository:\n\t%s\n", r.Config.URL, err)
				} else {
					fmt.Fprintf(m.Out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", r.Config.Name, r.Config.URL, err)
				}
			} else {
				// For those dependencies that are not known to helm and using a
				// generated key name we display the repo url.
				if strings.HasPrefix(r.Config.Name, managerKeyPrefix) {
					fmt.Fprintf(m.Out, "...Successfully got an update from the %q chart repository\n", r.Config.URL)
				} else {
					fmt.Fprintf(m.Out, "...Successfully got an update from the %q chart repository\n", r.Config.Name)
				}
			}
			wg.Done()
		}(r)
	}
	wg.Wait()

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
func (m *Manager) findChartURL(name, version, repoURL string, repos map[string]*repo.ChartRepository) (url, username, password string, insecureskiptlsverify, passcredentialsall bool, caFile, certFile, keyFile string, err error) {
	if registry.IsOCI(repoURL) {
		return fmt.Sprintf("%s/%s:%s", repoURL, name, version), "", "", false, false, "", "", "", nil
	}

	for _, cr := range repos {

		if urlutil.Equal(repoURL, cr.Config.URL) {
			var entry repo.ChartVersions
			entry, err = findEntryByName(name, cr)
			if err != nil {
				// TODO: Where linting is skipped in this function we should
				// refactor to remove naked returns while ensuring the same
				// behavior
				//nolint:nakedret
				return
			}
			var ve *repo.ChartVersion
			ve, err = findVersionedEntry(version, entry)
			if err != nil {
				//nolint:nakedret
				return
			}
			url, err = normalizeURL(repoURL, ve.URLs[0])
			if err != nil {
				//nolint:nakedret
				return
			}
			username = cr.Config.Username
			password = cr.Config.Password
			passcredentialsall = cr.Config.PassCredentialsAll
			insecureskiptlsverify = cr.Config.InsecureSkipTLSverify
			caFile = cr.Config.CAFile
			certFile = cr.Config.CertFile
			keyFile = cr.Config.KeyFile
			//nolint:nakedret
			return
		}
	}
	url, err = repo.FindChartInRepoURL(repoURL, name, version, certFile, keyFile, caFile, m.Getters)
	if err == nil {
		return url, username, password, false, false, "", "", "", err
	}
	err = errors.Errorf("chart %s not found in %s: %s", name, repoURL, err)
	return url, username, password, false, false, "", "", "", err
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

	u2.RawPath = path.Join(u2.RawPath, urlOrPath)
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
	return os.WriteFile(dest, data, 0644)
}

// archive a dep chart from local directory and save it into destPath
func tarFromLocalDir(chartpath, name, repo, version, destPath string) (string, error) {
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

// The prefix to use for cache keys created by the manager for repo names
const managerKeyPrefix = "helm-manager-"

// key is used to turn a name, such as a repository url, into a filesystem
// safe name that is unique for querying. To accomplish this a unique hash of
// the string is used.
func key(name string) (string, error) {
	in := strings.NewReader(name)
	hash := crypto.SHA256.New()
	if _, err := io.Copy(hash, in); err != nil {
		return "", nil
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
