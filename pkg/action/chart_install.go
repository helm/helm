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
	"io"
	"time"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chartutil"
)

// ChartInstall performs a chart install operation.
type ChartInstall struct {
	cfg *Configuration

	ClientOnly       bool
	DryRun           bool
	DisableHooks     bool
	Replace          bool
	Wait             bool
	Devel            bool
	DependencyUpdate bool
	Timeout          time.Duration
	Namespace        string
	ReleaseName      string
	GenerateName     bool
	NameTemplate     string
	OutputDir        string
	Atomic           bool
	SkipCRDs         bool
	SubNotes         bool
	// APIVersions allows a manual set of supported API Versions to be passed
	// (for things like templating). These are ignored if ClientOnly is false
	APIVersions chartutil.VersionSet
}

// NewChartInstall creates a new ChartInstall object with the given configuration.
func NewChartInstall(cfg *Configuration) *ChartInstall {
	return &ChartInstall{
		cfg: cfg,
	}
}

// Run executes the chart install operation
func (a *ChartInstall) Run(out io.Writer, name, ref string) error {
	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}

	return a.cfg.RegistryClient.InstallChart(name, r)
}
