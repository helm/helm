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
	"helm.sh/helm/v3/pkg/release"
)

// Get is the action for checking a given release's information.
//
// It provides the implementation of 'helm get' and its respective subcommands (except `helm get values`).
type Get struct {
	cfg *Configuration

	// Initializing Version to 0 will get the latest revision of the release.
	Version int
}

// NewGet creates a new Get object with the given configuration.
func NewGet(cfg *Configuration) *Get {
	return &Get{
		cfg: cfg,
	}
}

// Run executes 'helm get' against the given release.
func (g *Get) Run(name string) (*release.Release, error) {
	if err := g.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	return g.cfg.releaseContent(name, g.Version)
}
