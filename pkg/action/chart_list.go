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
)

// ChartList performs a chart list operation.
type ChartList struct {
	cfg *Configuration
}

// NewChartList creates a new ChartList object with the given configuration.
func NewChartList(cfg *Configuration) *ChartList {
	return &ChartList{
		cfg: cfg,
	}
}

// Run executes the chart list operation
func (a *ChartList) Run(out io.Writer) error {
	return a.cfg.RegistryClient.PrintChartTable()
}
