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

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chart"
)

// ChartSave performs a chart save operation.
type ChartSave struct {
	cfg *Configuration
}

// NewChartSave creates a new ChartSave object with the given configuration.
func NewChartSave(cfg *Configuration) *ChartSave {
	return &ChartSave{
		cfg: cfg,
	}
}

// Run executes the chart save operation
func (a *ChartSave) Run(out io.Writer, ch *chart.Chart, ref string) error {
	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}

	// If no tag is present, use the chart version
	if r.Tag == "" {
		r.Tag = ch.Metadata.Version
	}

	return a.cfg.RegistryClient.SaveChart(ch, r)
}
