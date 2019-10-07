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
	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/release"
)

// History is the action for checking the release's ledger.
//
// It provides the implementation of 'helm history'.
type History struct {
	cfg *Configuration

	Max     int
	Version int
}

// NewHistory creates a new History object with the given configuration.
func NewHistory(cfg *Configuration) *History {
	return &History{
		cfg: cfg,
	}
}

// Run executes 'helm history' against the given release.
func (h *History) Run(name string) ([]*release.Release, error) {
	if err := h.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	if err := validateReleaseName(name); err != nil {
		return nil, errors.Errorf("release name is invalid: %s", name)
	}

	h.cfg.Log("getting history for release %s", name)
	return h.cfg.Releases.History(name)
}
