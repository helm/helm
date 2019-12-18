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
	"helm.sh/helm/v3/pkg/chartutil"
)

// GetValues is the action for checking a given release's values.
//
// It provides the implementation of 'helm get values'.
type GetValues struct {
	cfg *Configuration

	Version   int
	AllValues bool
}

// NewGetValues creates a new GetValues object with the given configuration.
func NewGetValues(cfg *Configuration) *GetValues {
	return &GetValues{
		cfg: cfg,
	}
}

// Run executes 'helm get values' against the given release.
func (g *GetValues) Run(name string) (map[string]interface{}, error) {
	if err := g.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	rel, err := g.cfg.releaseContent(name, g.Version)
	if err != nil {
		return nil, err
	}

	// If the user wants all values, compute the values and return.
	if g.AllValues {
		cfg, err := chartutil.CoalesceValues(rel.Chart, rel.Config)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return rel.Config, nil
}
