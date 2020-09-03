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

// Status is the action for checking the deployment status of releases.
//
// It provides the implementation of 'helm status'.
type Status struct {
	cfg *Configuration

	Version int

	// If true, display description to output format,
	// only affect print type table.
	// TODO Helm 4: Remove this flag and output the description by default.
	ShowDescription bool
}

// NewStatus creates a new Status object with the given configuration.
func NewStatus(cfg *Configuration) *Status {
	return &Status{
		cfg: cfg,
	}
}

// Run executes 'helm status' against the given release.
func (s *Status) Run(name string) (*release.Release, error) {
	if err := s.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	return s.cfg.releaseContent(name, s.Version)
}
