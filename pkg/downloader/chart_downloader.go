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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/fileutil"
	ifs "helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/internal/urlutil"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
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
	RegistryClient   *registry.Client
	RepositoryConfig string
	RepositoryCache  string

	// ContentCache is the location where Cache stores its files by default
	// In previous versions of Helm the charts were put in the RepositoryCache. The
	// repositories and charts are stored in 2 different caches.
	ContentCache string

	// Cache specifies the cache implementation to use.
	Cache Cache
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
	if c.Cache == nil {
		if c.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		c.Cache = &DiskCache{Root: c.ContentCache}
		slog.Debug("set up default downloader cache")
	}
	hash, u, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	g, err := c.Getters.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Check the cache for the content. Otherwise download it.
	// Note, this process will pull from the cache but does not automatically populate
	// the cache with the file it downloads.
	var data *bytes.Buffer
	var found bool
	var digest []byte
	var digest32 [32]byte
	if hash != "" {
		// if there is a hash, populate the other formats
		digest, err = hex.DecodeString(hash)
		if err != nil {
			return "", nil, err
		}
		copy(digest32[:], digest)
		if pth, err := c.Cache.Get(digest32, CacheChart); err == nil {
			fdata, err := os.ReadFile(pth)
			if err == nil {
				found = true
				data = bytes.NewBuffer(fdata)
				slog.Debug("found chart in cache", "id", hash)
			}
		}
	}

	if !found {
		c.Options = append(c.Options, getter.WithAcceptHeader("application/gzip,application/octet-stream"))

		data, err = g.Get(u.String(), c.Options...)
		if err != nil {
			return "", nil, err
		}
	}

	name := filepath.Base(u.Path)
	if u.Scheme == registry.OCIScheme {
		idx := strings.LastIndexByte(name, ':')
		name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
	}

	destfile := filepath.Join(dest, name)
	if err := fileutil.AtomicWriteFile(destfile, data, 0644); err != nil {
		return destfile, nil, err
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {
		found = false
		var body *bytes.Buffer
		if hash != "" {
			if pth, err := c.Cache.Get(digest32, CacheProv); err == nil {
				fdata, err := os.ReadFile(pth)
				if err == nil {
					found = true
					body = bytes.NewBuffer(fdata)
					slog.Debug("found provenance in cache", "id", hash)
				}
			}
		}
		if !found {
			body, err = g.Get(u.String() + ".prov")
			if err != nil {
				if c.Verify == VerifyAlways {
					return destfile, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return destfile, ver, nil
			}
		}
		provfile := destfile + ".prov"
		if err := fileutil.AtomicWriteFile(provfile, body, 0644); err != nil {
			return destfile, nil, err
		}

		if c.Verify != VerifyLater {
			ver, err = VerifyChart(destfile, destfile+".prov", c.Keyring)
			if err != nil {
				// Fail always in this case, since it means the verification step
				// failed.
				return destfile, ver, err
			}
		}
	}
	return destfile, ver, nil
}

// DownloadToCache retrieves resources while using a content based cache.
func (c *ChartDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	if c.Cache == nil {
		if c.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		c.Cache = &DiskCache{Root: c.ContentCache}
		slog.Debug("set up default downloader cache")
	}

	digestString, u, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	g, err := c.Getters.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	c.Options = append(c.Options, getter.WithAcceptHeader("application/gzip,application/octet-stream"))

	// Check the cache for the file
	digest, err := hex.DecodeString(digestString)
	if err != nil {
		return "", nil, fmt.Errorf("unable to decode digest: %w", err)
	}
	var digest32 [32]byte
	copy(digest32[:], digest)

	var pth string
	// only fetch from the cache if we have a digest
	if len(digest) > 0 {
		pth, err = c.Cache.Get(digest32, CacheChart)
		if err == nil {
			slog.Debug("found chart in cache", "id", digestString)
		}
	}
	if len(digest) == 0 || err != nil {
		slog.Debug("attempting to download chart", "ref", ref, "version", version)
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}

		// Get file not in the cache
		data, gerr := g.Get(u.String(), c.Options...)
		if gerr != nil {
			return "", nil, gerr
		}

		// Generate the digest
		if len(digest) == 0 {
			digest32 = sha256.Sum256(data.Bytes())
		}

		pth, err = c.Cache.Put(digest32, data, CacheChart)
		if err != nil {
			return "", nil, err
		}
		slog.Debug("put downloaded chart in cache", "id", hex.EncodeToString(digest32[:]))
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {

		ppth, err := c.Cache.Get(digest32, CacheProv)
		if err == nil {
			slog.Debug("found provenance in cache", "id", digestString)
		} else {
			if !os.IsNotExist(err) {
				return pth, ver, err
			}

			body, err := g.Get(u.String() + ".prov")
			if err != nil {
				if c.Verify == VerifyAlways {
					return pth, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(c.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return pth, ver, nil
			}

			ppth, err = c.Cache.Put(digest32, body, CacheProv)
			if err != nil {
				return "", nil, err
			}
			slog.Debug("put downloaded provenance file in cache", "id", hex.EncodeToString(digest32[:]))
		}

		if c.Verify != VerifyLater {

			// provenance files pin to a specific name so this needs to be accounted for
			// when verifying.
			// Note, this does make an assumption that the name/version is unique to a
			// hash when a provenance file is used. If this isn't true, this section of code
			// will need to be reworked.
			name := filepath.Base(u.Path)
			if u.Scheme == registry.OCIScheme {
				idx := strings.LastIndexByte(name, ':')
				name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
			}

			// Copy chart to a known location with the right name for verification and then
			// clean it up.
			tmpdir := filepath.Dir(filepath.Join(c.ContentCache, "tmp"))
			if err := os.MkdirAll(tmpdir, 0755); err != nil {
				return pth, ver, err
			}
			tmpfile := filepath.Join(tmpdir, name)
			err = ifs.CopyFile(pth, tmpfile)
			if err != nil {
				return pth, ver, err
			}
			// Not removing the tmp dir itself because a concurrent process may be using it
			defer os.RemoveAll(tmpfile)

			ver, err = VerifyChart(tmpfile, ppth, c.Keyring)
			if err != nil {
				// Fail always in this case, since it means the verification step
				// failed.
				return pth, ver, err
			}
		}
	}
	return pth, ver, nil
}

// ResolveChartVersion resolves a chart reference to a URL.
//
// It returns:
// - A hash of the content if available
// - The URL and sets the ChartDownloader's Options that can fetch the URL using the appropriate Getter.
// - An error if there is one
//
// A reference may be an HTTP URL, an oci reference URL, a 'reponame/chartname'
// reference, or a local path.
//
// A version is a SemVer string (1.2.3-beta.1+f334a6789).
//
//   - For fully qualified URLs, the version will be ignored (since URLs aren't versioned)
//   - For a chart reference
//   - If version is non-empty, this will return the URL for that version
//   - If version is empty, this will return the URL for the latest version
//   - If no version can be found, an error is returned
//
// TODO: support OCI hash
func (c *ChartDownloader) ResolveChartVersion(ref, version string) (string, *url.URL, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", nil, fmt.Errorf("invalid chart URL format: %s", ref)
	}

	if registry.IsOCI(u.String()) {
		if c.RegistryClient == nil {
			return "", nil, fmt.Errorf("unable to lookup ref %s at version '%s', missing registry client", ref, version)
		}

		digest, OCIref, err := c.RegistryClient.ValidateReference(ref, version, u)
		return digest, OCIref, err
	}

	rf, err := loadRepoConfig(c.RepositoryConfig)
	if err != nil {
		return "", u, err
	}

	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// In this case, we have to find the parent repo that contains this chart
		// URL. And this is an unfortunate problem, as it requires actually going
		// through each repo cache file and finding a matching URL. But basically
		// we want to find the repo in case we have special SSL cert config
		// for that repo.

		rc, err := c.scanReposForURL(ref, rf)
		if err != nil {
			// If there is no special config, return the default HTTP client and
			// swallow the error.
			if err == ErrNoOwnerRepo {
				// Make sure to add the ref URL as the URL for the getter
				c.Options = append(c.Options, getter.WithURL(ref))
				return "", u, nil
			}
			return "", u, err
		}

		// If we get here, we don't need to go through the next phase of looking
		// up the URL. We have it already. So we just set the parameters and return.
		c.Options = append(
			c.Options,
			getter.WithURL(rc.URL),
		)
		if rc.CertFile != "" || rc.KeyFile != "" || rc.CAFile != "" {
			c.Options = append(c.Options, getter.WithTLSClientConfig(rc.CertFile, rc.KeyFile, rc.CAFile))
		}
		if rc.Username != "" && rc.Password != "" {
			c.Options = append(
				c.Options,
				getter.WithBasicAuth(rc.Username, rc.Password),
				getter.WithPassCredentialsAll(rc.PassCredentialsAll),
			)
		}
		return "", u, nil
	}

	// See if it's of the form: repo/path_to_chart
	p := strings.SplitN(u.Path, "/", 2)
	if len(p) < 2 {
		return "", u, fmt.Errorf("non-absolute URLs should be in form of repo_name/path_to_chart, got: %s", u)
	}

	repoName := p[0]
	chartName := p[1]
	rc, err := pickChartRepositoryConfigByName(repoName, rf.Repositories)
	if err != nil {
		return "", u, err
	}

	// Now that we have the chart repository information we can use that URL
	// to set the URL for the getter.
	c.Options = append(c.Options, getter.WithURL(rc.URL))

	r, err := repo.NewChartRepository(rc, c.Getters)
	if err != nil {
		return "", u, err
	}

	if r != nil && r.Config != nil {
		if r.Config.CertFile != "" || r.Config.KeyFile != "" || r.Config.CAFile != "" {
			c.Options = append(c.Options, getter.WithTLSClientConfig(r.Config.CertFile, r.Config.KeyFile, r.Config.CAFile))
		}
		if r.Config.Username != "" && r.Config.Password != "" {
			c.Options = append(c.Options,
				getter.WithBasicAuth(r.Config.Username, r.Config.Password),
				getter.WithPassCredentialsAll(r.Config.PassCredentialsAll),
			)
		}
	}

	// Next, we need to load the index, and actually look up the chart.
	idxFile := filepath.Join(c.RepositoryCache, helmpath.CacheIndexFile(r.Config.Name))
	i, err := repo.LoadIndexFile(idxFile)
	if err != nil {
		return "", u, fmt.Errorf("no cached repo found. (try 'helm repo update'): %w", err)
	}

	cv, err := i.Get(chartName, version)
	if err != nil {
		return "", u, fmt.Errorf("chart %q matching %s not found in %s index. (try 'helm repo update'): %w", chartName, version, r.Config.Name, err)
	}

	if len(cv.URLs) == 0 {
		return "", u, fmt.Errorf("chart %q has no downloadable URLs", ref)
	}

	// TODO: Seems that picking first URL is not fully correct
	resolvedURL, err := repo.ResolveReferenceURL(rc.URL, cv.URLs[0])
	if err != nil {
		return cv.Digest, u, fmt.Errorf("invalid chart URL format: %s", ref)
	}

	loc, err := url.Parse(resolvedURL)
	return cv.Digest, loc, err
}

// VerifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func VerifyChart(path, provfile, keyring string) (*provenance.Verification, error) {
	// For now, error out if it's not a tar file.
	switch fi, err := os.Stat(path); {
	case err != nil:
		return nil, err
	case fi.IsDir():
		return nil, errors.New("unpacked charts cannot be verified")
	case !isTar(path):
		return nil, errors.New("chart must be a tgz file")
	}

	if _, err := os.Stat(provfile); err != nil {
		return nil, fmt.Errorf("could not load provenance file %s: %w", provfile, err)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load keyring: %w", err)
	}

	// Read archive and provenance files
	archiveData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart archive: %w", err)
	}
	provData, err := os.ReadFile(provfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	return sig.Verify(archiveData, provData, filepath.Base(path))
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
				return nil, fmt.Errorf("no URL found for repository %s", name)
			}
			return rc, nil
		}
	}
	return nil, fmt.Errorf("repo %s not found", name)
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
			return nil, fmt.Errorf("no cached repo found. (try 'helm repo update'): %w", err)
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
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return r, nil
}
