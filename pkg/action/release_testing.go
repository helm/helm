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
	"time"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/release"
)

// ReleaseTesting is the action for testing a release.
//
// It provides the implementation of 'helm test'.
type ReleaseTesting struct {
	cfg *Configuration

	Timeout time.Duration
	Cleanup bool
}

// NewReleaseTesting creates a new ReleaseTesting object with the given configuration.
func NewReleaseTesting(cfg *Configuration) *ReleaseTesting {
	return &ReleaseTesting{
		cfg: cfg,
	}
}

// Run executes 'helm test' against the given release.
func (r *ReleaseTesting) Run(name string) error {
	if err := validateReleaseName(name); err != nil {
		return errors.Errorf("releaseTest: Release name is invalid: %s", name)
	}

	// finds the non-deleted release with the given name
	rel, err := r.cfg.Releases.Last(name)
	if err != nil {
		return err
	}

	if err := r.cfg.execHook(rel, release.HookTest, r.Timeout); err != nil {
		r.cfg.Releases.Update(rel)
		return err
	}

	return r.cfg.Releases.Update(rel)
}
