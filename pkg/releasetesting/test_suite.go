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

package releasetesting

import (
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"helm.sh/helm/pkg/release"
	util "helm.sh/helm/pkg/releaseutil"
)

// TestSuite what tests are run, results, and metadata
type TestSuite struct {
	StartedAt   time.Time
	CompletedAt time.Time
	Tests       []*release.Test
	Results     []*release.TestRun
}

type test struct {
	name            string
	manifest        string
	expectedSuccess bool
	result          *release.TestRun
}

// NewTestSuite takes a release object and returns a TestSuite object with test definitions
//  extracted from the release
func NewTestSuite(rel *release.Release) *TestSuite {
	return &TestSuite{
		Tests:   rel.Tests,
		Results: []*release.TestRun{},
	}
}

// Run executes tests in a test suite and stores a result within a given environment
func (ts *TestSuite) Run(env *Environment) error {
	ts.StartedAt = time.Now()

	if len(ts.Tests) == 0 {
		env.streamMessage("No Tests Found", release.TestRunUnknown)
	}

	for _, t := range ts.Tests {
		test, err := newTest(t)
		if err != nil {
			return err
		}

		test.result.StartedAt = time.Now()
		if err := env.streamRunning(test.name); err != nil {
			return err
		}
		test.result.Status = release.TestRunRunning

		resourceCreated := true
		if err := env.createTestPod(test); err != nil {
			resourceCreated = false
			if streamErr := env.streamError(test.result.Info); streamErr != nil {
				return err
			}
		}

		resourceCleanExit := true
		status := v1.PodUnknown
		if resourceCreated {
			status, err = env.getTestPodStatus(test)
			if err != nil {
				resourceCleanExit = false
				if streamErr := env.streamError(test.result.Info); streamErr != nil {
					return streamErr
				}
			}
		}

		if resourceCreated && resourceCleanExit {
			if err := test.assignTestResult(status); err != nil {
				return err
			}

			if err := env.streamResult(test.result); err != nil {
				return err
			}
		}

		test.result.CompletedAt = time.Now()
		ts.Results = append(ts.Results, test.result)
	}

	ts.CompletedAt = time.Now()
	return nil
}

func (t *test) assignTestResult(podStatus v1.PodPhase) error {
	switch podStatus {
	case v1.PodSucceeded:
		if t.expectedSuccess {
			t.result.Status = release.TestRunSuccess
		} else {
			t.result.Status = release.TestRunFailure
		}
	case v1.PodFailed:
		if !t.expectedSuccess {
			t.result.Status = release.TestRunSuccess
		} else {
			t.result.Status = release.TestRunFailure
		}
	default:
		t.result.Status = release.TestRunUnknown
	}

	return nil
}

func newTest(ts *release.Test) (*test, error) {
	var sh util.SimpleHead
	err := yaml.Unmarshal([]byte(ts.Manifest), &sh)
	if err != nil {
		return nil, err
	}

	if sh.Kind != "Pod" {
		return nil, errors.Errorf("%s is not a pod", sh.Metadata.Name)
	}

	name := strings.TrimSuffix(sh.Metadata.Name, ",")
	return &test{
		name:            name,
		manifest:        ts.Manifest,
		expectedSuccess: ts.ExpectSuccess,
		result: &release.TestRun{
			Name: name,
		},
	}, nil
}
