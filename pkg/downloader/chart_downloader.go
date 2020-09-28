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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/fileutil"
	"helm.sh/helm/v3/internal/urlutil"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"
)

// VerificationStrategy describes a strategy for determining whether to verify a chart.
type VerificationStrategy int

const (
	// VerifyNever will skip all verification of a chart.
	VerifyNever VerificationStrategy = iota
	// VerifyIfPossible will attempt a verification, it will not error if verification
	// data is missing. But it will not stop processing if verification fails.
	VerifyIfPossible
	// VerifyAlways will always attempt a verification, and will fail if the
	// verification fails.
	VerifyAlways
	// VerifyLater will fetch verification data, but not do any verification.
	// This is to accommodate the case where another step of the process will
	// perform verification.
	VerifyLater
)

const (
	provenanceFileExtension = ".prov"
)

// ErrNoOwnerRepo indicates that a given chart URL can't be found in any repos.
var ErrNoOwnerRepo = errors.New("could not find a repo containing the given URL")

// ChartDownloader handles downloading a chart.
//
// It is capable of performing verifications on charts as well.
type ChartDownloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// Getter collection for the operation
	Getters getter.Providers
	// Options provide parameters to be passed along to the Getter being initialized.
	Options          []getter.Option
	RepositoryConfig string
	RepositoryCache  string
	ChartCache       string
}

// DownloadTo retrieves a chart. Depending on the settings, it may also download a provenance file.
//
// If Verify is set to VerifyNever, the verification will be nil.
// If Verify is set to VerifyIfPossible, this will return a verification (or nil on failure), and print a warning on failure.
// If Verify is set to VerifyAlways, this will return a verification or an error if the verification fails.
// If Verify is set to VerifyLater, this will download the prov file (if it exists), but not verify it.
//
// For VerifyNever and VerifyIfPossible, the Verification may be empty.
//
// Returns a string path to the location where the file was downloaded and a verification
// (if provenance was verified), or an error if something bad happened.
func (c *ChartDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	chartPath, ver, err := c.Fetch(ref, version)
	if err != nil {
		return "", ver, err
	}

	// Copy the chart from cache to dest path, using the standard name (not the cache key)
	dstPath, err := CopyChart(chartPath, dest)
	return dstPath, ver, err
}

// CopyChart copies a chart to a destination path. If provenance file is available, it will also be copied to the same parent dir.
func CopyChart(srcPath, dstDir string) (string, error) {
	dstPath := filepath.Join(dstDir, path.Base(srcPath))
	if err := fileutil.AtomicCopyFile(srcPath, dstPath, 0644); err != nil {
		return dstPath, err
	}

	// Even though provenance is already verified on c.Fetch(), we still need to copy the file
	// This is the expected behavior on `helm pull` for example, and it is used on tests.
	chartProvenancePath := srcPath + provenanceFileExtension
	if _, err := os.Stat(chartProvenancePath); !os.IsNotExist(err) {
		return dstPath, fileutil.AtomicCopyFile(chartProvenancePath, dstPath+provenanceFileExtension, 0644)
	}

	return dstPath, nil
}

// Returns the URL for the provenance file of a chart
func provenanceURL(chart *url.URL) (*url.URL, error) {
	u, err := url.Parse(chart.String())
	if err != nil {
		return nil, err
	}

	u.Path += provenanceFileExtension
	return u, nil
}

// Fetch retrieves a chart. Depending on the settings, it may also download a provenance file.
//
// If Verify is set to VerifyNever, the verification will be nil.
// If Verify is set to VerifyIfPossible, this will return a verification (or nil on failure), and print a warning on failure.
// If Verify is set to VerifyAlways, this will return a verification or an error if the verification fails.
// If Verify is set to VerifyLater, this will download the prov file (if it exists), but not verify it.
//
// For VerifyNever and VerifyIfPossible, the Verification may be empty.
//
// Returns a string path to the location where the file was downloaded and a verification
// (if provenance was verified), or an error if something bad happened.
func (c *ChartDownloader) Fetch(ref, version string) (string, *provenance.Verification, error) {
	// Error if fails to dowload chart or fails to download provenance (when VerifyAlways)
	chartPath, err := c.downloadChartFiles(ref, version)
	if err != nil {
		return chartPath, nil, err
	}

	// Verify chart according to policy
	ver, err := c.provenanceVerificationPolicy(chartPath)
	return chartPath, ver, err
}

// provenanceVerificationPolicy applies the provenance verification policy.
// Returns a provenance verification or an error if something bad happened.
// See VerificationStrategy
func (c *ChartDownloader) provenanceVerificationPolicy(chartPath string) (*provenance.Verification, error) {
	// Nothing to verify
	if c.Verify == VerifyNever || c.Verify == VerifyLater {
		return &provenance.Verification{}, nil
	}

	// It is not possible to verify because provenance file is not available
	chartProvenancePath := chartPath + provenanceFileExtension
	if _, err := os.Stat(chartProvenancePath); os.IsNotExist(err) && c.Verify == VerifyIfPossible {
		return &provenance.Verification{}, nil
	}

	// It is possible, or required to verify
	return VerifyChart(chartPath, c.Keyring)
}

// downloadChartFiles downloads the chart identified by ref and version. It may also download its provenance file.
// Downloaded files are stored in ChartCache.
func (c *ChartDownloader) downloadChartFiles(ref, version string) (string, error) {
	u, repo, err := c.resolveChartVersion(ref, version)
	if err != nil {
		return "", err
	}

	g, err := c.Getters.ByScheme(u.Scheme)
	if err != nil {
		return "", err
	}

	// ChartCache is the workspace for chart downloads
	// TODO: implement --no-cache by making it set settings.ChartCache to a tmpdir
	if c.ChartCache == "" {
		return "", errors.New("unexpected empty chart cache path")
	}

	resourceURL := u.String()
	dstPath, err := c.cachePathFor(repo, resourceURL)
	if err != nil {
		return "", err
	}

	if err := c.cachedDownload(g, resourceURL, dstPath); err != nil {
		return dstPath, err
	}

	// No need to download the provenance file
	if c.Verify == VerifyNever {
		return dstPath, nil
	}

	// Download provenance file
	provU, err := provenanceURL(u)
	if err == nil {
		provDestPath := dstPath + provenanceFileExtension
		err = c.cachedDownload(g, provU.String(), provDestPath)
	}

	// Only returns errors if verification policy is Always
	if c.Verify == VerifyAlways && err != nil {
		return dstPath, errors.Errorf("failed to fetch provenance %q", provU.String())
	}

	// But log failures to download provenance file
	if err != nil {
		fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
	}

	return dstPath, nil
}

// cachedDownload downloads a resource to a destination
// TODO: implement caching policy
func (c *ChartDownloader) cachedDownload(g getter.Getter, resourceURL, dstPath string) error {
	if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
		return nil // nothing to do
	}

	data, err := g.Get(resourceURL, c.Options...)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	return fileutil.AtomicWriteFile(dstPath, data, 0644)
}

// Returns the chart cache path for a given chart of a repo.
// TODO: might be vulnerable to path traversals but I don't see any safe implementation in helm codebase
// https://github.com/golang/go/issues/20126
func (c *ChartDownloader) cachePathFor(repo *repo.Entry, chartURL string) (string, error) {
	repoName := "-" // represents an 'unknown' repo
	if repo != nil && repo.Name != "" {
		repoName = repo.Name
	}

	cacheKey, err := c.cacheKey(chartURL)
	if err != nil {
		return "", err
	}

	return filepath.Join(c.ChartCache, repoName, cacheKey), nil
}

// cacheKey returns a cache key that uniquely identifies a chart resource and is human readable.
// It also preserves the filename to make copy operations easier.
func (c *ChartDownloader) cacheKey(href string) (string, error) {
	baseURI, fileName := path.Split(href)
	digest, err := c.digest(baseURI)
	if err != nil {
		return "", err
	}

	return filepath.Join(digest, fileName), nil
}

// digest calculates a SHA1 message digest
func (c *ChartDownloader) digest(src string) (string, error) {
	hash := sha1.New()
	if _, err := io.WriteString(hash, src); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ResolveChartVersion resolves a chart reference to a URL.
//
// It returns the URL and sets the ChartDownloader's Options that can fetch
// the URL using the appropriate Getter.
//
// A reference may be an HTTP URL, a 'reponame/chartname' reference, or a local path.
//
// A version is a SemVer string (1.2.3-beta.1+f334a6789).
//
//	- For fully qualified URLs, the version will be ignored (since URLs aren't versioned)
//	- For a chart reference
//		* If version is non-empty, this will return the URL for that version
//		* If version is empty, this will return the URL for the latest version
//		* If no version can be found, an error is returned
func (c *ChartDownloader) ResolveChartVersion(ref, version string) (u *url.URL, err error) {
	u, _, err = c.resolveChartVersion(ref, version)
	return
}

// resolveChartVersion resolves a chart reference to a URL and a repo entry.
func (c *ChartDownloader) resolveChartVersion(ref, version string) (*url.URL, *repo.Entry, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, nil, errors.Errorf("invalid chart URL format: %s", ref)
	}
	c.Options = append(c.Options, getter.WithURL(ref))

	rf, err := loadRepoConfig(c.RepositoryConfig)
	if err != nil {
		return u, nil, err
	}

	// TODO: assess lookup performance
	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// In this case, we have to find the parent repo that contains this chart
		// URL. And this is an unfortunate problem, as it requires actually going
		// through each repo cache file and finding a matching URL. But basically
		// we want to find the repo in case we have special SSL cert config
		// for that repo.

		rc, err := c.scanReposForURL(ref, rf)

		// If there is no special config, return the default HTTP client and
		// swallow the error.
		if err == ErrNoOwnerRepo {
			return u, nil, nil
		}

		if err != nil {
			return u, nil, err
		}

		// If we get here, we don't need to go through the next phase of looking
		// up the URL. We have it already. So we just set the parameters and return.
		c.setOptionsFromRepo(rc)
		return u, rc, nil
	}

	// See if it's of the form: repo/path_to_chart
	p := strings.SplitN(u.Path, "/", 2)
	if len(p) < 2 {
		return u, nil, errors.Errorf("non-absolute URLs should be in form of repo_name/path_to_chart, got: %s", u)
	}

	repoName := p[0]
	chartName := p[1]
	rc, err := pickChartRepositoryConfigByName(repoName, rf.Repositories)
	if err != nil {
		return u, nil, err
	}

	// Validates the repo.Entry found (valid URL with a valid schema)
	r, err := repo.NewChartRepository(rc, c.Getters)
	if err != nil {
		return u, rc, err
	}

	c.setOptionsFromRepo(r.Config)

	// Next, we need to load the index, and actually look up the chart.
	idxFile := filepath.Join(c.RepositoryCache, helmpath.CacheIndexFile(r.Config.Name))
	i, err := repo.LoadIndexFile(idxFile)
	if err != nil {
		return u, rc, errors.Wrap(err, "no cached repo found. (try 'helm repo update')")
	}

	cv, err := i.Get(chartName, version)
	if err != nil {
		return u, rc, errors.Wrapf(err, "chart %q matching %s not found in %s index. (try 'helm repo update')", chartName, version, r.Config.Name)
	}

	if len(cv.URLs) == 0 {
		return u, rc, errors.Errorf("chart %q has no downloadable URLs", ref)
	}

	// TODO: Seems that picking first URL is not fully correct
	u, err = url.Parse(cv.URLs[0])
	if err != nil {
		return u, rc, errors.Errorf("invalid chart URL format: %s", ref)
	}

	if u.IsAbs() {
		return u, rc, nil
	}

	// If the URL is relative (no scheme), prepend the chart repo's base URL
	repoURL, err := url.Parse(rc.URL)
	if err != nil {
		return repoURL, rc, err
	}

	// We need a trailing slash for ResolveReference to work, but make sure there isn't already one
	repoURL.Path = strings.TrimSuffix(repoURL.Path, "/") + "/"
	u = repoURL.ResolveReference(u)
	u.RawQuery = repoURL.Query().Encode()
	if _, err := getter.NewHTTPGetter(); err != nil {
		return repoURL, rc, err
	}

	return u, rc, nil
}

func (c *ChartDownloader) setOptionsFromRepo(rc *repo.Entry) {
	c.Options = append(c.Options, getter.WithURL(rc.URL)) // TODO: remove

	if rc.CertFile != "" || rc.KeyFile != "" || rc.CAFile != "" {
		c.Options = append(c.Options, getter.WithTLSClientConfig(rc.CertFile, rc.KeyFile, rc.CAFile))
	}

	if rc.Username != "" && rc.Password != "" {
		c.Options = append(c.Options, getter.WithBasicAuth(rc.Username, rc.Password))
	}
}

// VerifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func VerifyChart(path, keyring string) (*provenance.Verification, error) {
	// For now, error out if it's not a tar file.
	switch fi, err := os.Stat(path); {
	case err != nil:
		return nil, err
	case fi.IsDir():
		return nil, errors.New("unpacked charts cannot be verified")
	case !isTar(path):
		return nil, errors.New("chart must be a tgz file")
	}

	provfile := path + provenanceFileExtension
	if _, err := os.Stat(provfile); err != nil {
		return nil, errors.Wrapf(err, "could not load provenance file %s", provfile)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to load keyring")
	}
	return sig.Verify(path, provfile)
}

// isTar tests whether the given file is a tar file.
//
// Currently, this simply checks extension, since a subsequent function will
// untar the file and validate its binary format.
func isTar(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".tgz")
}

func pickChartRepositoryConfigByName(name string, cfgs []*repo.Entry) (*repo.Entry, error) {
	for _, rc := range cfgs {
		if rc.Name == name {
			if rc.URL == "" {
				return nil, errors.Errorf("no URL found for repository %s", name)
			}
			return rc, nil
		}
	}
	return nil, errors.Errorf("repo %s not found", name)
}

// scanReposForURL scans all repos to find which repo contains the given URL.
//
// This will attempt to find the given URL in all of the known repositories files.
//
// If the URL is found, this will return the repo entry that contained that URL.
//
// If all of the repos are checked, but the URL is not found, an ErrNoOwnerRepo
// error is returned.
//
// Other errors may be returned when repositories cannot be loaded or searched.
//
// Technically, the fact that a URL is not found in a repo is not a failure indication.
// Charts are not required to be included in an index before they are valid. So
// be mindful of this case.
//
// The same URL can technically exist in two or more repositories. This algorithm
// will return the first one it finds. Order is determined by the order of repositories
// in the repositories.yaml file.
func (c *ChartDownloader) scanReposForURL(u string, rf *repo.File) (*repo.Entry, error) {
	// FIXME: This is far from optimal. Larger installations and index files will
	// incur a performance hit for this type of scanning.
	for _, rc := range rf.Repositories {
		r, err := repo.NewChartRepository(rc, c.Getters)
		if err != nil {
			return nil, err
		}

		idxFile := filepath.Join(c.RepositoryCache, helmpath.CacheIndexFile(r.Config.Name))
		i, err := repo.LoadIndexFile(idxFile)
		if err != nil {
			return nil, errors.Wrap(err, "no cached repo found. (try 'helm repo update')")
		}

		for _, entry := range i.Entries {
			for _, ver := range entry {
				for _, dl := range ver.URLs {
					if urlutil.Equal(u, dl) {
						return rc, nil
					}
				}
			}
		}
	}
	// This means that there is no repo file for the given URL.
	return nil, ErrNoOwnerRepo
}

func loadRepoConfig(file string) (*repo.File, error) {
	r, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(errors.Cause(err)) {
		return nil, err
	}
	return r, nil
}
