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

// Chart-specific artifact downloading capabilities.
package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ifs "helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/provenance"
	"helm.sh/helm/v4/pkg/registry"
)

// ChartDownloader handles downloading charts with chart-specific features.
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
	// repositories and charts are stored in 2 difference caches.
	ContentCache string

	// Cache specifies the cache implementation to use.
	Cache Cache

	// Internal unified downloader
	downloader *Downloader
}

// initDownloader initializes the internal downloader if needed
func (c *ChartDownloader) initDownloader() {
	if c.downloader == nil {
		// Initialize cache if needed
		if c.Cache == nil && c.ContentCache != "" {
			c.Cache = &DiskCache{Root: c.ContentCache}
		}

		c.downloader = &Downloader{
			Out:          c.Out,
			Verify:       c.Verify,
			Keyring:      c.Keyring,
			Getters:      c.Getters,
			Options:      c.Options,
			Cache:        c.Cache,
			ContentCache: c.ContentCache,
		}
		c.downloader.SetRepositoryConfig(c.RepositoryConfig, c.RepositoryCache)
		if c.RegistryClient != nil {
			c.downloader.SetRegistryClient(c.RegistryClient)
		}
	}
}

// DownloadTo retrieves a chart. Depending on the settings, it may also download a provenance file.
func (c *ChartDownloader) DownloadTo(ref, version, dest string) (string, *provenance.Verification, error) {
	c.initDownloader()
	return c.downloader.Download(ref, version, dest, TypeChart)
}

// DownloadToCache downloads a chart to the cache.
//
// TODO: This method doesn't call DownloadTo directly due to legacy complexity from the original
// chart downloader's intricate cache management patterns that weren't easily unified. Unlike
// PluginDownloader which has the simpler pattern (DownloadToCache calls DownloadTo), this
// implementation maintains separate logic for:
// - Content-addressable cache paths vs user-specified destinations
// - Complex verification involving temporary files for cached content
// - Different performance optimizations for cache hits vs direct downloads
//
// Investigate whether we can simplify and unify these patterns to match PluginDownloader's
// cleaner approach where DownloadToCache(ref, version) calls DownloadTo(ref, version, cache).
func (c *ChartDownloader) DownloadToCache(ref, version string) (string, *provenance.Verification, error) {
	if c.Cache == nil {
		if c.ContentCache == "" {
			return "", nil, errors.New("content cache must be set")
		}
		c.Cache = &DiskCache{Root: c.ContentCache}
	}

	digestString, u, err := c.ResolveChartVersion(ref, version)
	if err != nil {
		return "", nil, err
	}

	// Get the appropriate getter for this scheme
	g, err := c.Getters.ByScheme(u.Scheme)
	if err != nil {
		return "", nil, err
	}

	// Check the cache for the file
	digest, err := hex.DecodeString(digestString)
	if err != nil {
		return "", nil, err
	}
	var digest32 [sha256.Size]byte
	copy(digest32[:], digest)

	var pth string
	// only fetch from the cache if we have a digest
	if len(digest) > 0 {
		pth, err = c.Cache.Get(digest32, CacheChart)
		if err == nil {
			// Found in cache, check for verification
			ver := &provenance.Verification{}
			if c.Verify > VerifyNever {
				ppth, err := c.Cache.Get(digest32, CacheProv)
				if err == nil && c.Verify != VerifyLater {
					name := filepath.Base(u.Path)
					if u.Scheme == registry.OCIScheme {
						idx := strings.LastIndexByte(name, ':')
						name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
					}
					tmpdir := filepath.Dir(filepath.Join(c.ContentCache, "tmp"))
					if err := os.MkdirAll(tmpdir, 0755); err != nil {
						return pth, ver, err
					}
					tmpfile := filepath.Join(tmpdir, name)
					if err := ifs.CopyFile(pth, tmpfile); err != nil {
						return pth, ver, err
					}
					defer os.RemoveAll(tmpfile)
					ver, err = VerifyChart(tmpfile, ppth, c.Keyring)
					if err != nil {
						return pth, ver, err
					}
				}
			}
			return pth, ver, nil
		}
	}
	if len(digest) == 0 || err != nil {
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}

		// Get file not in the cache
		opts := append(c.Options, getter.WithURL(u.String()))
		data, gerr := g.Get(u.String(), opts...)
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
	}

	// If provenance is requested, verify it.
	ver := &provenance.Verification{}
	if c.Verify > VerifyNever {
		ppth, err := c.Cache.Get(digest32, CacheProv)
		if err != nil {
			if !os.IsNotExist(err) {
				return pth, ver, err
			}

			body, err := g.Get(u.String() + ".prov") // No options for provenance request (matches original implementation)
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
		}

		if c.Verify != VerifyLater {
			name := filepath.Base(u.Path)
			if u.Scheme == registry.OCIScheme {
				idx := strings.LastIndexByte(name, ':')
				name = fmt.Sprintf("%s-%s.tgz", name[:idx], name[idx+1:])
			}

			tmpdir := filepath.Dir(filepath.Join(c.ContentCache, "tmp"))
			if err := os.MkdirAll(tmpdir, 0755); err != nil {
				return pth, ver, err
			}
			tmpfile := filepath.Join(tmpdir, name)
			err = ifs.CopyFile(pth, tmpfile)
			if err != nil {
				return pth, ver, err
			}
			defer os.RemoveAll(tmpfile)

			ver, err = VerifyChart(tmpfile, ppth, c.Keyring)
			if err != nil {
				return pth, ver, err
			}
		}
	}
	return pth, ver, nil
}

// ResolveChartVersion resolves a chart reference to a URL.
// This delegates to the unified artifact downloader.
func (c *ChartDownloader) ResolveChartVersion(ref, version string) (string, *url.URL, error) {
	c.initDownloader()
	return c.downloader.ResolveArtifactVersion(ref, version, TypeChart)
}

// NewChartDownloader creates a new ChartDownloader.
func NewChartDownloader() *ChartDownloader {
	return &ChartDownloader{}
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

	if keyring == "" {
		keyring = defaultKeyring()
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

func defaultKeyring() string {
	return os.ExpandEnv("$PGP_KEYRING")
}
