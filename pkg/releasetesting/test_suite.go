/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/protobuf/ptypes/timestamp"
	"k8s.io/kubernetes/pkg/api"

	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/timeconv"
)

// TestSuite what tests are run, results, and metadata
type TestSuite struct {
	StartedAt     *timestamp.Timestamp
	CompletedAt   *timestamp.Timestamp
	TestManifests []string
	Results       []*release.TestRun
}

type test struct {
	manifest string
	result   *release.TestRun
}

// NewTestSuite takes a release object and returns a TestSuite object with test definitions
//  extracted from the release
func NewTestSuite(rel *release.Release) (*TestSuite, error) {
	testManifests, err := extractTestManifestsFromHooks(rel.Hooks, rel.Name)
	if err != nil {
		return nil, err
	}

	results := []*release.TestRun{}

	return &TestSuite{
		TestManifests: testManifests,
		Results:       results,
	}, nil
}

// Run executes tests in a test suite and stores a result within the context of a given environment
func (t *TestSuite) Run(env *Environment) error {
	t.StartedAt = timeconv.Now()

	for _, testManifest := range t.TestManifests {
		test, err := newTest(testManifest)
		if err != nil {
			return err
		}

		test.result.StartedAt = timeconv.Now()
		if err := env.streamRunning(test.result.Name); err != nil {
			return err
		}

		resourceCreated := true
		if err := t.createTestPod(test, env); err != nil {
			resourceCreated = false
			if streamErr := env.streamError(test.result.Info); streamErr != nil {
				return err
			}
		}

		resourceCleanExit := true
		status := api.PodUnknown
		if resourceCreated {
			status, err = t.getTestPodStatus(test, env)
			if err != nil {
				resourceCleanExit = false
				if streamErr := env.streamUnknown(test.result.Name, test.result.Info); streamErr != nil {
					return streamErr
				}
			}
		}

		if resourceCreated && resourceCleanExit && status == api.PodSucceeded {
			test.result.Status = release.TestRun_SUCCESS
			if streamErr := env.streamSuccess(test.result.Name); streamErr != nil {
				return streamErr
			}
		} else if resourceCreated && resourceCleanExit && status == api.PodFailed {
			test.result.Status = release.TestRun_FAILURE
			if streamErr := env.streamFailed(test.result.Name); streamErr != nil {
				return err
			}
		}

		test.result.CompletedAt = timeconv.Now()
		t.Results = append(t.Results, test.result)
	}

	t.CompletedAt = timeconv.Now()
	return nil
}

// NOTE: may want to move this function to pkg/tiller in the future
func filterHooksForTestHooks(hooks []*release.Hook, releaseName string) ([]*release.Hook, error) {
	testHooks := []*release.Hook{}
	notFoundErr := fmt.Errorf("no tests found for release %s", releaseName)

	if len(hooks) == 0 {
		return nil, notFoundErr
	}

	for _, h := range hooks {
		for _, e := range h.Events {
			if e == release.Hook_RELEASE_TEST_SUCCESS {
				testHooks = append(testHooks, h)
				continue
			}
		}
	}

	if len(testHooks) == 0 {
		return nil, notFoundErr
	}

	return testHooks, nil
}

// NOTE: may want to move this function to pkg/tiller in the future
func extractTestManifestsFromHooks(hooks []*release.Hook, releaseName string) ([]string, error) {
	testHooks, err := filterHooksForTestHooks(hooks, releaseName)
	if err != nil {
		return nil, err
	}

	tests := []string{}
	for _, h := range testHooks {
		individualTests := util.SplitManifests(h.Manifest)
		for _, t := range individualTests {
			tests = append(tests, t)
		}
	}
	return tests, nil
}

func newTest(testManifest string) (*test, error) {
	var sh util.SimpleHead
	err := yaml.Unmarshal([]byte(testManifest), &sh)
	if err != nil {
		return nil, err
	}

	if sh.Kind != "Pod" {
		return nil, fmt.Errorf("%s is not a pod", sh.Metadata.Name)
	}

	name := strings.TrimSuffix(sh.Metadata.Name, ",")
	return &test{
		manifest: testManifest,
		result: &release.TestRun{
			Name: name,
		},
	}, nil
}

func (t *TestSuite) createTestPod(test *test, env *Environment) error {
	b := bytes.NewBufferString(test.manifest)
	if err := env.KubeClient.Create(env.Namespace, b, env.Timeout, false); err != nil {
		log.Printf(err.Error())
		test.result.Info = err.Error()
		test.result.Status = release.TestRun_FAILURE
		return err
	}

	return nil
}

func (t *TestSuite) getTestPodStatus(test *test, env *Environment) (api.PodPhase, error) {
	b := bytes.NewBufferString(test.manifest)
	status, err := env.KubeClient.WaitAndGetCompletedPodPhase(env.Namespace, b, time.Duration(env.Timeout)*time.Second)
	if err != nil {
		log.Printf("Error getting status for pod %s: %s", test.result.Name, err)
		test.result.Info = err.Error()
		test.result.Status = release.TestRun_UNKNOWN
		return status, err
	}

	return status, err
}
