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

package action

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/pkg/chart/loader/archive"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

// Pull is the action for checking a given release's information.
//
// It provides the implementation of 'helm pull'.
type Pull struct {
	ChartPathOptions

	Settings *cli.EnvSettings // TODO: refactor this out of pkg/action

	Devel       bool
	Untar       bool
	VerifyLater bool
	UntarDir    string
	DestDir     string
	cfg         *Configuration
	// MaxChartSize is the maximum decompressed size of a chart in bytes
	MaxChartSize int64
	// MaxChartFileSize is the maximum size of a single file in a chart in bytes
	MaxChartFileSize int64
}

type PullOpt func(*Pull)

func WithConfig(cfg *Configuration) PullOpt {
	return func(p *Pull) {
		p.cfg = cfg
	}
}

// NewPull creates a new Pull with configuration options.
func NewPull(opts ...PullOpt) *Pull {
	p := &Pull{}
	for _, fn := range opts {
		fn(p)
	}

	return p
}

// SetRegistryClient sets the registry client on the pull configuration object.
func (p *Pull) SetRegistryClient(client *registry.Client) {
	p.cfg.RegistryClient = client
}

// Run executes 'helm pull' against the given release.
func (p *Pull) Run(chartRef string) (string, error) {
	var out strings.Builder

	c := downloader.ChartDownloader{
		Out:     &out,
		Keyring: p.Keyring,
		Verify:  downloader.VerifyNever,
		Getters: getter.All(p.Settings),
		Options: []getter.Option{
			getter.WithBasicAuth(p.Username, p.Password),
			getter.WithPassCredentialsAll(p.PassCredentialsAll),
			getter.WithTLSClientConfig(p.CertFile, p.KeyFile, p.CaFile),
			getter.WithInsecureSkipVerifyTLS(p.InsecureSkipTLSVerify),
			getter.WithPlainHTTP(p.PlainHTTP),
		},
		RegistryClient:   p.cfg.RegistryClient,
		RepositoryConfig: p.Settings.RepositoryConfig,
		RepositoryCache:  p.Settings.RepositoryCache,
		ContentCache:     p.Settings.ContentCache,
	}

	if registry.IsOCI(chartRef) {
		c.Options = append(c.Options,
			getter.WithRegistryClient(p.cfg.RegistryClient))
		c.RegistryClient = p.cfg.RegistryClient
	}

	if p.Verify {
		c.Verify = downloader.VerifyAlways
	} else if p.VerifyLater {
		c.Verify = downloader.VerifyLater
	}

	// If untar is set, we fetch to a tempdir, then untar and copy after
	// verification.
	dest := p.DestDir
	if p.Untar {
		var err error
		dest, err = os.MkdirTemp("", "helm-")
		if err != nil {
			return out.String(), fmt.Errorf("failed to untar: %w", err)
		}
		defer os.RemoveAll(dest)
	}

	downloadSourceRef := chartRef
	if p.RepoURL != "" {
		chartURL, err := repo.FindChartInRepoURL(
			p.RepoURL,
			chartRef,
			getter.All(p.Settings),
			repo.WithChartVersion(p.Version),
			repo.WithClientTLS(p.CertFile, p.KeyFile, p.CaFile),
			repo.WithUsernamePassword(p.Username, p.Password),
			repo.WithInsecureSkipTLSVerify(p.InsecureSkipTLSVerify),
			repo.WithPassCredentialsAll(p.PassCredentialsAll),
		)
		if err != nil {
			return out.String(), err
		}
		downloadSourceRef = chartURL
	}

	saved, v, err := c.DownloadTo(downloadSourceRef, p.Version, dest)
	if err != nil {
		return out.String(), err
	}

	if p.Verify {
		for name := range v.SignedBy.Identities {
			fmt.Fprintf(&out, "Signed by: %v\n", name)
		}
		fmt.Fprintf(&out, "Using Key With Fingerprint: %X\n", v.SignedBy.PrimaryKey.Fingerprint)
		fmt.Fprintf(&out, "Chart Hash Verified: %s\n", v.FileHash)
	}

	// After verification, untar the chart into the requested directory.
	if p.Untar {
		ud := p.UntarDir
		if !filepath.IsAbs(ud) {
			ud = filepath.Join(p.DestDir, ud)
		}
		// Let udCheck to check conflict file/dir without replacing ud when untarDir is the current directory(.).
		udCheck := ud
		if udCheck == "." {
			_, udCheck = filepath.Split(chartRef)
		} else {
			_, chartName := filepath.Split(chartRef)
			udCheck = filepath.Join(udCheck, chartName)
		}

		if _, err := os.Stat(udCheck); err != nil {
			if err := os.MkdirAll(udCheck, 0755); err != nil {
				return out.String(), fmt.Errorf("failed to untar (mkdir): %w", err)
			}
		} else {
			return out.String(), fmt.Errorf("failed to untar: a file or directory with the name %s already exists", udCheck)
		}

		opts := archive.DefaultOptions
		if p.MaxChartSize > 0 {
			opts.MaxDecompressedChartSize = p.MaxChartSize
		}
		if p.MaxChartFileSize > 0 {
			opts.MaxDecompressedFileSize = p.MaxChartFileSize
		}
		return out.String(), chartutil.ExpandFileWithOptions(ud, saved, opts)
	}
	return out.String(), nil
}
