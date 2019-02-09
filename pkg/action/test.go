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

	"k8s.io/helm/pkg/release"
	reltesting "k8s.io/helm/pkg/releasetesting"
)

// Test is the action for testing a given release.
//
// It provides the implementation of 'helm test'.
type Test struct {
	cfg *Configuration

	Timeout int64
	Cleanup bool
}

// NewTest creates a new Test object with the given configuration.
func NewTest(cfg *Configuration) *Test {
	return &Test{
		cfg: cfg,
	}
}

// Run executes 'helm test' against the given release.
func (t *Test) Run(name string) (<-chan *release.TestReleaseResponse, <-chan error) {
	errc := make(chan error, 1)
	if err := validateReleaseName(name); err != nil {
		errc <- errors.Errorf("releaseTest: Release name is invalid: %s", name)
		return nil, errc
	}

	// finds the non-deleted release with the given name
	rel, err := t.cfg.Releases.Last(name)
	if err != nil {
		errc <- err
		return nil, errc
	}

	ch := make(chan *release.TestReleaseResponse, 1)
	testEnv := &reltesting.Environment{
		Namespace:  rel.Namespace,
		KubeClient: t.cfg.KubeClient,
		Timeout:    t.Timeout,
		Messages:   ch,
	}
	t.cfg.Log("running tests for release %s", rel.Name)
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

		if t.Cleanup {
			testEnv.DeleteTestPods(tSuite.TestManifests)
		}

		if err := t.cfg.Releases.Update(rel); err != nil {
			t.cfg.Log("test: Failed to store updated release: %s", err)
		}
	}()
	return ch, errc
}
