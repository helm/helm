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
	reltesting "helm.sh/helm/pkg/releasetesting"
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
func (r *ReleaseTesting) Run(name string) (<-chan *release.TestReleaseResponse, <-chan error) {
	errc := make(chan error, 1)
	if err := validateReleaseName(name); err != nil {
		errc <- errors.Errorf("releaseTest: Release name is invalid: %s", name)
		return nil, errc
	}

	// finds the non-deleted release with the given name
	rel, err := r.cfg.Releases.Last(name)
	if err != nil {
		errc <- err
		return nil, errc
	}

	ch := make(chan *release.TestReleaseResponse, 1)
	testEnv := &reltesting.Environment{
		Namespace:  rel.Namespace,
		KubeClient: r.cfg.KubeClient,
		Timeout:    r.Timeout,
		Messages:   ch,
	}
	r.cfg.Log("running tests for release %s", rel.Name)
	tSuite := reltesting.NewTestSuite(rel)

	go func() {
		defer close(errc)
		defer close(ch)

		if err := tSuite.Run(testEnv); err != nil {
			errc <- errors.Wrapf(err, "error running test suite for %s", rel.Name)
			return
		}

		rel.Info.LastTestSuiteRun = &release.TestSuite{
			StartedAt:   tSuite.StartedAt,
			CompletedAt: tSuite.CompletedAt,
			Results:     tSuite.Results,
		}

		if r.Cleanup {
			testEnv.DeleteTestPods(tSuite.TestManifests)
		}

		if err := r.cfg.Releases.Update(rel); err != nil {
			r.cfg.Log("test: Failed to store updated release: %s", err)
		}
	}()
	return ch, errc
}
