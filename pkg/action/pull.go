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

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
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
}

type PullOpt func(*Pull)

func WithConfig(cfg *Configuration) PullOpt {
	return func(p *Pull) {
		p.cfg = cfg
	}
}

// NewPull creates a new Pull object.
func NewPull() *Pull {
	return NewPullWithOpts()
}

// NewPullWithOpts creates a new pull, with configuration options.
func NewPullWithOpts(opts ...PullOpt) *Pull {
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
			getter.WithInsecureSkipVerifyTLS(p.InsecureSkipTLSverify),
			getter.WithPlainHTTP(p.PlainHTTP),
		},
		RegistryClient:   p.cfg.RegistryClient,
		RepositoryConfig: p.Settings.RepositoryConfig,
		RepositoryCache:  p.Settings.RepositoryCache,
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
			return out.String(), errors.Wrap(err, "failed to untar")
		}
		defer os.RemoveAll(dest)
	}

	if p.RepoURL != "" {
		chartURL, err := repo.FindChartInAuthAndTLSAndPassRepoURL(p.RepoURL, p.Username, p.Password, chartRef, p.Version, p.CertFile, p.KeyFile, p.CaFile, p.InsecureSkipTLSverify, p.PassCredentialsAll, getter.All(p.Settings))
		if err != nil {
			return out.String(), err
		}
		chartRef = chartURL
	}

	saved, v, err := c.DownloadTo(chartRef, p.Version, dest)
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
				return out.String(), errors.Wrap(err, "failed to untar (mkdir)")
			}

		} else {
			return out.String(), errors.Errorf("failed to untar: a file or directory with the name %s already exists", udCheck)
		}

		return out.String(), chartutil.ExpandFile(ud, saved)
	}
	return out.String(), nil
}
