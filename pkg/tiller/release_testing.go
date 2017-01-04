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

package tiller

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/api"

	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/timeconv"
)

// change name to runReleaseTestSuite
func runReleaseTestSuite(hooks []*release.Hook, kube environment.KubeClient, name, namespace string, timeout int64) (*release.TestSuite, error) {

	suite := &release.TestSuite{}
	suite.LastRun = timeconv.Now()
	results := []*release.TestResult{}

	tests, err := prepareTests(hooks, name)
	if err != nil {
		return suite, err
	}

	for _, h := range tests {
		var sh simpleHead
		err := yaml.Unmarshal([]byte(h), &sh)
		if err != nil {
			//handle err better
			return nil, err
		}
		ts := &release.TestResult{Name: sh.Metadata.Name}

		// should this be lower? should we even be saving time to hook?
		// TODO: should be start time really
		ts.LastRun = timeconv.Now()

		resourceCreated := true
		b := bytes.NewBufferString(h)
		if err := kube.Create(namespace, b); err != nil {
			log.Printf("Could not create %s(%s): %v", ts.Name, sh.Kind, err)
			ts.Info = err.Error()
			//TODO: status option should be constant not random int
			ts.Status = 2
			resourceCreated = false
		}

		status := api.PodUnknown
		resourceCleanExit := true
		if resourceCreated {
			b.Reset()
			b.WriteString(h)
			status, err = kube.WaitAndGetCompletedPodStatus(namespace, b, time.Duration(timeout)*time.Second)
			if err != nil {
				log.Printf("Error getting status for %s(%s): %s", ts.Name, sh.Kind, err)
				ts.Info = err.Error()
				ts.Status = 0
				resourceCleanExit = false
			}
		}

		// TODO: maybe better suited as a switch statement and include
		//      PodUnknown, PodFailed, PodRunning, and PodPending scenarios
		if resourceCreated && resourceCleanExit && status == api.PodSucceeded {
			ts.Status = 1
		} else if resourceCreated && resourceCleanExit && status == api.PodFailed {
			ts.Status = 2
		}

		results = append(results, ts)
		log.Printf("Test %s(%s) complete", ts.Name, sh.Kind)

		//TODO: recordTests() - add test results to configmap with standardized name
	}

	suite.Results = results
	log.Printf("Finished running test suite for %s", name)

	return suite, nil
}

func filterTests(hooks []*release.Hook, releaseName string) ([]*release.Hook, error) {
	testHooks := []*release.Hook{}
	notFoundErr := fmt.Errorf("no tests found for release %s", releaseName)
	if len(hooks) == 0 {
		return nil, notFoundErr
	}

	code, ok := events[releaseTest]
	if !ok {
		return nil, fmt.Errorf("unknown hook %q", releaseTest)
	}

	found := false
	for _, h := range hooks {
		for _, e := range h.Events {
			if e == code {
				found = true
				testHooks = append(testHooks, h)
				continue
			}
		}
	}

	//TODO: probably don't need to check found
	if found == false && len(testHooks) == 0 {
		return nil, notFoundErr
	}

	return testHooks, nil
}

func prepareTests(hooks []*release.Hook, releaseName string) ([]string, error) {
	testHooks, err := filterTests(hooks, releaseName)
	if err != nil {
		return nil, err
	}

	tests := []string{}
	for _, h := range testHooks {
		individualTests := splitManifests(h.Manifest)
		for _, t := range individualTests {
			tests = append(tests, t)
		}
	}
	return tests, nil
}
