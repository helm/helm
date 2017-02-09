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

	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tiller/environment"
)

// Environment encapsulates information about where test suite executes and returns results
type Environment struct {
	Namespace  string
	KubeClient environment.KubeClient
	Stream     services.ReleaseService_RunReleaseTestServer
	Timeout    int64
}

func (env *Environment) streamRunning(name string) error {
	msg := "RUNNING: " + name
	return env.streamMessage(msg)
}

func (env *Environment) streamError(info string) error {
	msg := "ERROR: " + info
	return env.streamMessage(msg)
}

func (env *Environment) streamFailed(name string) error {
	msg := fmt.Sprintf("FAILED: %s, run `kubectl logs %s --namespace %s` for more info", name, name, env.Namespace)
	return env.streamMessage(msg)
}

func (env *Environment) streamSuccess(name string) error {
	msg := fmt.Sprintf("PASSED: %s", name)
	return env.streamMessage(msg)
}

func (env *Environment) streamUnknown(name, info string) error {
	msg := fmt.Sprintf("UNKNOWN: %s: %s", name, info)
	return env.streamMessage(msg)
}

func (env *Environment) streamMessage(msg string) error {
	resp := &services.TestReleaseResponse{Msg: msg}
	return env.Stream.Send(resp)
}

// DeleteTestPods deletes resources given in testManifests
func (env *Environment) DeleteTestPods(testManifests []string) {
	for _, testManifest := range testManifests {
		err := env.KubeClient.Delete(env.Namespace, bytes.NewBufferString(testManifest))
		if err != nil {
			env.streamError(err.Error())
		}
	}
}
