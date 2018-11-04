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

package tiller

import (
	"github.com/pkg/errors"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	reltesting "k8s.io/helm/pkg/releasetesting"
)

// RunReleaseTest runs pre-defined tests stored as hooks on a given release
func (s *ReleaseServer) RunReleaseTest(req *hapi.TestReleaseRequest) (<-chan *hapi.TestReleaseResponse, <-chan error) {
	errc := make(chan error, 1)
	if err := validateReleaseName(req.Name); err != nil {
		errc <- errors.Errorf("releaseTest: Release name is invalid: %s", req.Name)
		return nil, errc
	}

	// finds the non-deleted release with the given name
	rel, err := s.Releases.Last(req.Name)
	if err != nil {
		errc <- err
		return nil, errc
	}

	ch := make(chan *hapi.TestReleaseResponse, 1)
	testEnv := &reltesting.Environment{
		Namespace:  rel.Namespace,
		KubeClient: s.KubeClient,
		Timeout:    req.Timeout,
		Mesages:    ch,
	}
	s.Log("running tests for release %s", rel.Name)
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

		if req.Cleanup {
			testEnv.DeleteTestPods(tSuite.TestManifests)
		}

		if err := s.Releases.Update(rel); err != nil {
			s.Log("test: Failed to store updated release: %s", err)
		}
	}()
	return ch, errc
}
