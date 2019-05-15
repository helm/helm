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
	"bytes"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"

	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/pkg/release"
)

// Environment encapsulates information about where test suite executes and returns results
type Environment struct {
	Namespace  string
	KubeClient kube.Interface
	Messages   chan *release.TestReleaseResponse
	Timeout    time.Duration
}

func (env *Environment) createTestPod(test *test) error {
	b := bytes.NewBufferString(test.manifest)
	if err := env.KubeClient.Create(b); err != nil {
		test.result.Info = err.Error()
		test.result.Status = release.TestRunFailure
		return err
	}

	return nil
}

func (env *Environment) getTestPodStatus(test *test) (v1.PodPhase, error) {
	status, err := env.KubeClient.WaitAndGetCompletedPodPhase(test.name, env.Timeout)
	if err != nil {
		log.Printf("Error getting status for pod %s: %s", test.result.Name, err)
		test.result.Info = err.Error()
		test.result.Status = release.TestRunUnknown
		return status, err
	}

	return status, err
}

func (env *Environment) streamResult(r *release.TestRun) error {
	switch r.Status {
	case release.TestRunSuccess:
		if err := env.streamSuccess(r.Name); err != nil {
			return err
		}
	case release.TestRunFailure:
		if err := env.streamFailed(r.Name); err != nil {
			return err
		}

	default:
		if err := env.streamUnknown(r.Name, r.Info); err != nil {
			return err
		}
	}
	return nil
}

func (env *Environment) streamRunning(name string) error {
	msg := "RUNNING: " + name
	return env.streamMessage(msg, release.TestRunRunning)
}

func (env *Environment) streamError(info string) error {
	msg := "ERROR: " + info
	return env.streamMessage(msg, release.TestRunFailure)
}

func (env *Environment) streamFailed(name string) error {
	msg := fmt.Sprintf("FAILED: %s, run `kubectl logs %s --namespace %s` for more info", name, name, env.Namespace)
	return env.streamMessage(msg, release.TestRunFailure)
}

func (env *Environment) streamSuccess(name string) error {
	msg := fmt.Sprintf("PASSED: %s", name)
	return env.streamMessage(msg, release.TestRunSuccess)
}

func (env *Environment) streamUnknown(name, info string) error {
	msg := fmt.Sprintf("UNKNOWN: %s: %s", name, info)
	return env.streamMessage(msg, release.TestRunUnknown)
}

func (env *Environment) streamMessage(msg string, status release.TestRunStatus) error {
	resp := &release.TestReleaseResponse{Msg: msg, Status: status}
	env.Messages <- resp
	return nil
}

// DeleteTestPods deletes resources given in testManifests
func (env *Environment) DeleteTestPods(testManifests []string) {
	for _, testManifest := range testManifests {
		err := env.KubeClient.Delete(bytes.NewBufferString(testManifest))
		if err != nil {
			env.streamError(err.Error())
		}
	}
}
