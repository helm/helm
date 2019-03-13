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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/cli"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/repo"
)

// Pull is the action for checking a given release's information.
//
// It provides the implementation of 'helm pull'.
type Pull struct {
	ChartPathOptions

	Settings cli.EnvSettings // TODO: refactor this out of pkg/action

	Devel       bool
	Untar       bool
	VerifyLater bool
	UntarDir    string
	DestDir     string
}

// NewPull creates a new Pull object with the given configuration.
func NewPull() *Pull {
	return &Pull{}
}

// Run executes 'helm pull' against the given release.
func (p *Pull) Run(chartRef string) (string, error) {
	var out strings.Builder

	c := downloader.ChartDownloader{
		HelmHome: p.Settings.Home,
		Out:      &out,
		Keyring:  p.Keyring,
		Verify:   downloader.VerifyNever,
		Getters:  getter.All(p.Settings),
		Username: p.Username,
		Password: p.Password,
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
		dest, err = ioutil.TempDir("", "helm-")
		if err != nil {
			return out.String(), errors.Wrap(err, "failed to untar")
		}
		defer os.RemoveAll(dest)
	}

	if p.RepoURL != "" {
		chartURL, err := repo.FindChartInAuthRepoURL(p.RepoURL, p.Username, p.Password, chartRef, p.Version, p.CertFile, p.KeyFile, p.CaFile, getter.All(p.Settings))
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
		fmt.Fprintf(&out, "Verification: %v\n", v)
	}

	// After verification, untar the chart into the requested directory.
	if p.Untar {
		ud := p.UntarDir
		if !filepath.IsAbs(ud) {
			ud = filepath.Join(p.DestDir, ud)
		}
		if fi, err := os.Stat(ud); err != nil {
			if err := os.MkdirAll(ud, 0755); err != nil {
				return out.String(), errors.Wrap(err, "failed to untar (mkdir)")
			}

		} else if !fi.IsDir() {
			return out.String(), errors.Errorf("failed to untar: %s is not a directory", ud)
		}

		return out.String(), chartutil.ExpandFile(ud, saved)
	}
	return out.String(), nil
}

func (p *Pull) AddFlags(f *pflag.FlagSet) {
	f.BoolVar(&p.Devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")
	f.BoolVar(&p.Untar, "untar", false, "if set to true, will untar the chart after downloading it")
	f.BoolVar(&p.VerifyLater, "prov", false, "fetch the provenance file, but don't perform verification")
	f.StringVar(&p.UntarDir, "untardir", ".", "if untar is specified, this flag specifies the name of the directory into which the chart is expanded")
	f.StringVarP(&p.DestDir, "destination", "d", ".", "location to write the chart. If this and tardir are specified, tardir is appended to this")
	p.ChartPathOptions.AddFlags(f)
}
