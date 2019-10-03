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
)

// ChartPull performs a chart pull operation.
type ChartPull struct {
	cfg *Configuration
}

// NewChartPull creates a new ChartPull object with the given configuration.
func NewChartPull(cfg *Configuration) *ChartPull {
	return &ChartPull{
		cfg: cfg,
	}
}

// Run executes the chart pull operation
func (a *ChartPull) Run(out io.Writer, ref string) error {
	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}
	return a.cfg.RegistryClient.PullChart(r)
}
