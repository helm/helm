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
	"io"
	"path/filepath"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chartutil"
)

// ChartExport performs a chart export operation.
type ChartExport struct {
	cfg *Configuration

	Destination string
}

// NewChartExport creates a new ChartExport object with the given configuration.
func NewChartExport(cfg *Configuration) *ChartExport {
	return &ChartExport{
		cfg: cfg,
	}
}

// Run executes the chart export operation
func (a *ChartExport) Run(out io.Writer, ref string) error {
	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}

	ch, err := a.cfg.RegistryClient.LoadChart(r)
	if err != nil {
		return err
	}

	// Save the chart to local destination directory
	err = chartutil.SaveDir(ch, a.Destination)
	if err != nil {
		return err
	}

	d := filepath.Join(a.Destination, ch.Metadata.Name)
	fmt.Fprintf(out, "Exported chart to %s/\n", d)
	return nil
}
