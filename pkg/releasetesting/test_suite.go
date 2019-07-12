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

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/pkg/hooks"
	"helm.sh/helm/pkg/release"
	util "helm.sh/helm/pkg/releaseutil"
)

// TestSuite what tests are run, results, and metadata
type TestSuite struct {
	StartedAt     time.Time
	CompletedAt   time.Time
	TestManifests []string
	Results       []*release.TestRun
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
		TestManifests: extractTestManifestsFromHooks(rel.Hooks),
		Results:       []*release.TestRun{},
	}
}

// Run executes tests in a test suite and stores a result within a given environment
func (ts *TestSuite) Run(env *Environment) error {
	ts.StartedAt = time.Now()

	if len(ts.TestManifests) == 0 {
		// TODO: make this better, adding test run status on test suite is weird
		env.streamMessage("No Tests Found", release.TestRunUnknown)
	}

	for _, testManifest := range ts.TestManifests {
		test, err := newTest(testManifest)
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

func expectedSuccess(hookTypes []string) (bool, error) {
	for _, hookType := range hookTypes {
		hookType = strings.ToLower(strings.TrimSpace(hookType))
		if hookType == hooks.ReleaseTestSuccess {
			return true, nil
		} else if hookType == hooks.ReleaseTestFailure {
			return false, nil
		}
	}
	return false, errors.Errorf("no %s or %s hook found", hooks.ReleaseTestSuccess, hooks.ReleaseTestFailure)
}

func extractTestManifestsFromHooks(h []*release.Hook) []string {
	testHooks := hooks.FilterTestHooks(h)

	tests := []string{}
	for _, h := range testHooks {
		individualTests := util.SplitManifests(h.Manifest)
		for _, t := range individualTests {
			tests = append(tests, t)
		}
	}
	return tests
}

func newTest(testManifest string) (*test, error) {
	var sh util.SimpleHead
	err := yaml.Unmarshal([]byte(testManifest), &sh)
	if err != nil {
		return nil, err
	}

	if sh.Kind != "Pod" {
		return nil, errors.Errorf("%s is not a pod", sh.Metadata.Name)
	}

	hookTypes := sh.Metadata.Annotations[hooks.HookAnno]
	expected, err := expectedSuccess(strings.Split(hookTypes, ","))
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(sh.Metadata.Name, ",")
	return &test{
		name:            name,
		manifest:        testManifest,
		expectedSuccess: expected,
		result: &release.TestRun{
			Name: name,
		},
	}, nil
}
