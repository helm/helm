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

func streamRunning(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := "RUNNING: " + name
	err := streamMessage(msg, stream)
	return err
}

func streamError(info string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := "ERROR: " + info
	err := streamMessage(msg, stream)
	return err
}

func streamFailed(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := fmt.Sprintf("FAILED: %s, run `kubectl logs %s` for more info", name, name)
	err := streamMessage(msg, stream)
	return err
}

func streamSuccess(name string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := fmt.Sprintf("PASSED: %s", name)
	err := streamMessage(msg, stream)
	return err
}

func streamUnknown(name, info string, stream services.ReleaseService_RunReleaseTestServer) error {
	msg := fmt.Sprintf("UNKNOWN: %s: %s", name, info)
	err := streamMessage(msg, stream)
	return err
}

func streamMessage(msg string, stream services.ReleaseService_RunReleaseTestServer) error {
	resp := &services.TestReleaseResponse{Msg: msg}
	err := stream.Send(resp)
	return err
}
