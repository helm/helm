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
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

// Type represents the type of artifact being downloaded.
type Type string

const (
	TypeChart  Type = "chart"
	TypePlugin Type = "plugin"
	// Future artifact types beyond charts and plugins can be added here
)

// VerificationStrategy describes a strategy for determining whether to verify an downloader.
type VerificationStrategy int

const (
	// VerifyNever will skip all verification of an downloader.
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

// ErrNoOwnerRepo indicates that a given artifact URL can't be found in any repos.
var ErrNoOwnerRepo = errors.New("could not find a repo containing the given URL")

// Downloader handles downloading artifacts with verification and caching.
type Downloader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Verify indicates what verification strategy to use.
	Verify VerificationStrategy
	// Keyring is the keyring file used for verification.
	Keyring string
	// Getters provide protocol handling for different URL schemes.
	Getters getter.Providers
	// Options provide parameters to be passed along to Getters.
	Options []getter.Option
	// ContentCache is the location where Cache stores its files by default.
	ContentCache string
	// Cache specifies the cache implementation to use.
	Cache Cache

	// Configuration for repository-based transports
	repositoryConfig string
	repositoryCache  string
	// Configuration for OCI-based transports
	registryClient *registry.Client
}

// Download retrieves an downloader. Depending on the settings, it may also download a provenance file.
//
// If Verify is set to VerifyNever, the verification will be nil.
// If Verify is set to VerifyIfPossible, this will return a verification (or nil on failure), and print a warning on failure.
// If Verify is set to VerifyAlways, this will return a verification or an error if the verification fails.
// If Verify is set to VerifyLater, this will download the prov file (if it exists), but not verify it.
//
// Returns a string path to the location where the file was downloaded and a verification
// (if provenance was verified), or an error if something bad happened.
func (d *Downloader) Download(ref, version, dest string, artifactType Type) (string, *provenance.Verification, error) {
	if d.Cache == nil {
		if d.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		d.Cache = &DiskCache{Root: d.ContentCache}
		slog.Debug("set up default downloader cache")
	}

	hash, u, err := d.ResolveArtifactVersion(ref, version, artifactType)
	if err != nil {
		return "", nil, err
	}

	if err := d.validateArtifactTypeForScheme(u.Scheme, artifactType); err != nil {
		return "", nil, err
	}

	t, err := d.Getters.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Check the cache for the content. Otherwise download it.
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
		if pth, err := d.Cache.Get(digest32, CacheChart); err == nil {
			fdata, err := os.ReadFile(pth)
			if err == nil {
				found = true
				data = bytes.NewBuffer(fdata)
				slog.Debug("found artifact in cache", "id", hash, "type", artifactType)
			}
		}
	}

	if !found {
		d.Options = append(d.Options, getter.WithAcceptHeader("application/gzip,application/octet-stream"))
		opts := append(d.Options, getter.WithURL(u.String()))
		data, err = t.Get(u.String(), opts...)
		if err != nil {
			return "", nil, err
		}
	}

	name := d.generateArtifactName(u, artifactType)
	destfile := filepath.Join(dest, name)
	if err := fileutil.AtomicWriteFile(destfile, data, 0644); err != nil {
		return destfile, nil, err
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if d.Verify > VerifyNever {
		found = false
		var body *bytes.Buffer
		if hash != "" {
			if pth, err := d.Cache.Get(digest32, CacheProv); err == nil {
				fdata, err := os.ReadFile(pth)
				if err == nil {
					found = true
					body = bytes.NewBuffer(fdata)
					slog.Debug("found provenance in cache", "id", hash, "type", artifactType)
				}
			}
		}
		if !found {
			body, err = t.Get(u.String() + ".prov") // No options for provenance request (matches original implementation)
			if err != nil {
				if d.Verify == VerifyAlways {
					return destfile, ver, fmt.Errorf("failed to fetch provenance %q", u.String()+".prov")
				}
				fmt.Fprintf(d.Out, "WARNING: Verification not found for %s: %s\n", ref, err)
				return destfile, ver, nil
			}
		}
		provfile := destfile + ".prov"
		if err := fileutil.AtomicWriteFile(provfile, body, 0644); err != nil {
			return destfile, nil, err
		}

		if d.Verify != VerifyLater {
			ver, err = d.VerifyArtifact(destfile, destfile+".prov")
			if err != nil {
				return destfile, ver, err
			}
		}
	}
	return destfile, ver, nil
}

// SetRepositoryConfig sets the repository configuration for repository-based transports.
func (d *Downloader) SetRepositoryConfig(config, cache string) {
	d.repositoryConfig = config
	d.repositoryCache = cache
}

// SetRegistryClient sets the registry client for OCI-based transports.
func (d *Downloader) SetRegistryClient(client *registry.Client) {
	d.registryClient = client
}

// configureGetter configures a getter based on its supported interfaces.
func (d *Downloader) validateArtifactTypeForScheme(scheme string, artifactType Type) error {
	g, err := d.Getters.ByScheme(scheme)
	if err != nil {
		return err
	}

	// If getter implements getter.Restricted, check restrictions
	if restricted, ok := g.(getter.Restricted); ok {
		supportedTypes := restricted.RestrictToArtifactTypes()
		if supportedTypes == nil {
			return nil
		}
		for _, t := range supportedTypes {
			if t == string(artifactType) {
				return nil
			}
		}
		return fmt.Errorf("scheme %s does not support artifact type %s", scheme, artifactType)
	}

	// Default: support all types
	return nil
}

// generateArtifactName generates the filename for the downloaded downloader.
func (d *Downloader) generateArtifactName(u *url.URL, artifactType Type) string {
	name := filepath.Base(u.Path)

	// Handle OCI references
	if u.Scheme == registry.OCIScheme {
		idx := strings.LastIndexByte(name, ':')
		if idx >= 0 {
			name = fmt.Sprintf("%s-%s", name[:idx], name[idx+1:])
		}
	}

	// Add appropriate extension based on artifact type
	switch artifactType {
	case TypeChart:
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	case TypePlugin:
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	default:
		// Default to tgz for unknown types
		if !strings.HasSuffix(name, ".tgz") {
			name += ".tgz"
		}
	}

	return name
}

// ResolveArtifactVersion resolves an artifact reference to a URL.
//
// It returns:
// - A hash of the content if available
// - The URL for downloading the artifact
// - An error if resolution fails
//
// A reference may be an HTTP URL, an OCI reference URL, a 'reponame/artifactname'
// reference, or a local path.
//
// A version is a SemVer string (1.2.3-beta.1+f334a6789).
func (d *Downloader) ResolveArtifactVersion(ref, version string, artifactType Type) (string, *url.URL, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", nil, fmt.Errorf("invalid artifact URL format: %s", ref)
	}

	// Handle OCI references - supported for all artifact types
	if registry.IsOCI(u.String()) {
		if d.registryClient == nil {
			// In testing or when registry client is not configured,
			// treat OCI URLs as direct URLs to allow testing
			return "", u, nil
		}

		digest, OCIref, err := d.registryClient.ValidateReference(ref, version, u)
		return digest, OCIref, err
	}

	// Handle direct URLs (http/https/file) - supported for all artifact types
	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// For direct URLs, version is typically ignored since URLs are specific
		// But we could validate that the URL contains the expected version string
		return "", u, nil
	}

	// Handle repository-based references (reponame/artifactname)
	switch artifactType {
	case TypeChart:
		return d.resolveChartFromRepository(ref, version, u)

	case TypePlugin:
		// Plugins use modern OCI-based distribution, not repository-based like charts
		// Repository-based plugin distribution is not planned due to scalability issues
		// with the chart repository index model
		return "", nil, fmt.Errorf("repository-based plugin distribution is not supported - use OCI references instead (oci://registry/plugin:version)")

	default:
		return "", nil, fmt.Errorf("unknown artifact type: %s", artifactType)
	}
}

// VerifyArtifact verifies an artifact using its provenance file.
func (d *Downloader) VerifyArtifact(artifactPath, provPath string) (*provenance.Verification, error) {
	// Check if artifact exists and is valid format
	switch fi, err := os.Stat(artifactPath); {
	case err != nil:
		return nil, err
	case fi.IsDir():
		return nil, errors.New("unpacked artifacts cannot be verified")
	case !strings.HasSuffix(artifactPath, ".tgz"):
		return nil, errors.New("artifact must be a tgz file")
	}

	if _, err := os.Stat(provPath); err != nil {
		return nil, fmt.Errorf("could not load provenance file %s: %w", provPath, err)
	}

	sig, err := provenance.NewFromKeyring(d.Keyring, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load keyring: %w", err)
	}

	// Read artifact and provenance files
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}
	provData, err := os.ReadFile(provPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance file: %w", err)
	}

	return sig.Verify(artifactData, provData, filepath.Base(artifactPath))
}

// loadRepoConfig loads the repository configuration file.
func loadRepoConfig(file string) (*repo.File, error) {
	r, err := repo.LoadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return r, nil
}

// pickChartRepositoryConfigByName returns the repository configuration for the given name.
func pickChartRepositoryConfigByName(name string, cfgs []*repo.Entry) (*repo.Entry, error) {
	for _, rc := range cfgs {
		if rc.Name == name {
			if rc.URL == "" {
				return nil, fmt.Errorf("no URL found for repository %s", name)
			}
			return rc, nil
		}
	}
	return nil, fmt.Errorf("repository name (%s) not found. You must add the repository before using it", name)
}

// scanReposForURL scans all repositories for a URL that matches the given reference.
func (d *Downloader) scanReposForURL(ref string, rf *repo.File) (*repo.Entry, error) {
	// Scan all of the repositories looking for a URL match
	for _, rc := range rf.Repositories {
		if strings.Index(ref, rc.URL) == 0 {
			return rc, nil
		}
	}

	// This means that the URL is not associated with a known repository
	return nil, ErrNoOwnerRepo
}

// resolveChartFromRepository resolves a chart reference from a repository.
func (d *Downloader) resolveChartFromRepository(ref, version string, u *url.URL) (string, *url.URL, error) {
	rf, err := loadRepoConfig(d.repositoryConfig)
	if err != nil {
		return "", u, err
	}

	// Handle direct URLs that might be in a known repository
	if u.IsAbs() && len(u.Host) > 0 && len(u.Path) > 0 {
		// Try to find the parent repo that contains this chart URL
		rc, err := d.scanReposForURL(ref, rf)
		if err != nil {
			// If there is no special config, return the URL for direct download
			if err == ErrNoOwnerRepo {
				// This will be handled by the transport layer
				return "", u, nil
			}
			return "", u, err
		}

		// Configure transport options based on repository config
		d.configureRepositoryOptions(rc)
		return "", u, nil
	}

	// Handle repository-based references in the form "reponame/chartname"
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

	// Configure transport options for this repository
	d.configureRepositoryOptions(rc)

	// Load the repository index to find the chart
	idxFile := filepath.Join(d.repositoryCache, helmpath.CacheIndexFile(rc.Name))
	i, err := repo.LoadIndexFile(idxFile)
	if err != nil {
		return "", u, fmt.Errorf("no cached repo found. (try 'helm repo update'): %w", err)
	}

	cv, err := i.Get(chartName, version)
	if err != nil {
		return "", u, fmt.Errorf("chart %q matching %s not found in %s index. (try 'helm repo update'): %w", chartName, version, rc.Name, err)
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

// configureRepositoryOptions configures transport options for a repository.
func (d *Downloader) configureRepositoryOptions(rc *repo.Entry) {
	if rc == nil {
		return
	}

	// Add TLS configuration if available
	if rc.CertFile != "" || rc.KeyFile != "" || rc.CAFile != "" {
		d.Options = append(d.Options, getter.WithTLSClientConfig(rc.CertFile, rc.KeyFile, rc.CAFile))
		if rc.InsecureSkipTLSverify {
			d.Options = append(d.Options, getter.WithInsecureSkipVerifyTLS(rc.InsecureSkipTLSverify))
		}
	}

	// Add basic auth if available
	if rc.Username != "" && rc.Password != "" {
		d.Options = append(d.Options, getter.WithBasicAuth(rc.Username, rc.Password))
	}

	// Add pass credentials all flag if set
	if rc.PassCredentialsAll {
		d.Options = append(d.Options, getter.WithPassCredentialsAll(true))
	}

	// Set the repository URL for the getter
	if rc.URL != "" {
		d.Options = append(d.Options, getter.WithURL(rc.URL))
	}
}
