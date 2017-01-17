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
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/timeconv"
)

//TODO: testSuiteRunner.Run()
//struct testSuiteRunner {
//suite *release.TestSuite,
//tests []string,
//kube environemtn.KubeClient,
//timeout int64
////stream or output channel
//}

func runReleaseTests(tests []string, rel *release.Release, kube environment.KubeClient, stream services.ReleaseService_RunReleaseTestServer, timeout int64) (*release.TestSuite, error) {
	results := []*release.TestRun{}

	//TODO: add results to test suite
	suite := &release.TestSuite{}
	suite.StartedAt = timeconv.Now()

	for _, h := range tests {
		var sh simpleHead
		err := yaml.Unmarshal([]byte(h), &sh)
		if err != nil {
			return nil, err
		}

		if sh.Kind != "Pod" {
			return nil, fmt.Errorf("%s is not a pod", sh.Metadata.Name)
		}

		ts := &release.TestRun{Name: sh.Metadata.Name}
		ts.StartedAt = timeconv.Now()
		if err := streamRunning(ts.Name, stream); err != nil {
			return nil, err
		}

		resourceCreated := true
		b := bytes.NewBufferString(h)
		if err := kube.Create(rel.Namespace, b); err != nil {
			resourceCreated = false
			msg := fmt.Sprintf("ERROR: %s", err)
			log.Printf(msg)
			ts.Info = err.Error()
			ts.Status = release.TestRun_FAILURE
			if streamErr := streamMessage(msg, stream); streamErr != nil {
				return nil, err
			}
		}

		status := api.PodUnknown
		resourceCleanExit := true
		if resourceCreated {
			b.Reset()
			b.WriteString(h)
			status, err = kube.WaitAndGetCompletedPodPhase(rel.Namespace, b, time.Duration(timeout)*time.Second)
			if err != nil {
				resourceCleanExit = false
				log.Printf("Error getting status for pod %s: %s", ts.Name, err)
				ts.Info = err.Error()
				ts.Status = release.TestRun_UNKNOWN
				if streamErr := streamFailed(ts.Name, stream); streamErr != nil {
					return nil, err
				}
			}
		}

		// TODO: maybe better suited as a switch statement and include
		//      PodUnknown, PodFailed, PodRunning, and PodPending scenarios
		if resourceCreated && resourceCleanExit && status == api.PodSucceeded {
			ts.Status = release.TestRun_SUCCESS
			if streamErr := streamSuccess(ts.Name, stream); streamErr != nil {
				return nil, streamErr
			}
		} else if resourceCreated && resourceCleanExit && status == api.PodFailed {
			ts.Status = release.TestRun_FAILURE
			if streamErr := streamFailed(ts.Name, stream); streamErr != nil {
				return nil, err
			}
		}

		results = append(results, ts)
		log.Printf("Test %s completed", ts.Name)

		//TODO: recordTests() - add test results to configmap with standardized name
	}

	suite.Results = results
	//TODO: delete flag
	log.Printf("Finished running test suite for %s", rel.Name)

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
	if !found && len(testHooks) == 0 {
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

func streamRunning(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := "RUNNING: " + name
	if err := streamMessage(msg, stream); err != nil {
		return err
	}
	return nil
}

func streamFailed(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := fmt.Sprintf("FAILED: %s, run `kubectl logs %s` for more info", name, name)
	if err := streamMessage(msg, stream); err != nil {
		return err
	}
	return nil
}

func streamSuccess(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := fmt.Sprintf("PASSED: %s", name)
	if err := streamMessage(msg, stream); err != nil {
		return err
	}
	return nil
}

func streamMessage(msg string, stream services.ReleaseService_RunReleaseTestServer) error {
	resp := &services.TestReleaseResponse{Msg: msg}
	// TODO: handle err better
	if err := stream.Send(resp); err != nil {
		return err
	}

	return nil
}
