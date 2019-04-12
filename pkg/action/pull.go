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

	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/repo"
)

// Pull is the action for checking a given release's information.
//
// It provides the implementation of 'helm pull'.
type Pull struct {
	cfg *Configuration
	ChartPathOptions

	Settings cli.EnvSettings // TODO: refactor this out of pkg/action

	Devel       bool
	Untar       bool
	VerifyLater bool
	UntarDir    string
	DestDir     string
}

// NewPull creates a new Pull object with the given configuration.
func NewPull(cfg *Configuration) *Pull {
	return &Pull{cfg: cfg}
}

// Run executes 'helm pull' against the given release.
func (p *Pull) Run(chartRef string) (string, error) {
	var out strings.Builder

	ref, err := repo.ParseNameTag(chartRef, p.Version)
	if err != nil {
		return out.String(), err
	}

	if p.Verify {
		// TODO(bacongobbler): plug into pkg/repo or oras for signing during a pull
		//
		// see comment in pkg/repo/client.go#PullChart
	}

	if err := p.cfg.RegistryClient.PullChart(ref); err != nil {
		return out.String(), fmt.Errorf("failed to download %q: %v", ref.String(), err)
	}

	ch, err := p.cfg.RegistryClient.LoadChart(ref)
	if err != nil {
		return out.String(), err
	}

	dest := p.DestDir
	if p.Untar {
		var err error
		dest, err = ioutil.TempDir("", "helm-")
		if err != nil {
			return out.String(), errors.Wrap(err, "failed to untar")
		}
		defer os.RemoveAll(dest)
	}

	saved, err := chartutil.Save(ch, dest)
	if err != nil {
		return out.String(), err
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
