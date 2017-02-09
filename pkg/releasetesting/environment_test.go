/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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
	"errors"
	"io"
	"os"
	"testing"

	tillerEnv "k8s.io/helm/pkg/tiller/environment"
)

func TestDeleteTestPods(t *testing.T) {
	mockTestSuite := testSuiteFixture()
	mockTestEnv := newMockTestingEnvironment()
	mockTestEnv.KubeClient = newGetFailingKubeClient()

	mockTestEnv.DeleteTestPods(mockTestSuite.TestManifests)

	stream := mockTestEnv.Stream.(mockStream)
	if len(stream.messages) != 0 {
		t.Errorf("Expected 0 errors, got at least one: %v", stream.messages)
	}

	for _, testManifest := range mockTestSuite.TestManifests {
		if _, err := mockTestEnv.KubeClient.Get(mockTestEnv.Namespace, bytes.NewBufferString(testManifest)); err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestDeleteTestPodsFailingDelete(t *testing.T) {
	mockTestSuite := testSuiteFixture()
	mockTestEnv := newMockTestingEnvironment()
	mockTestEnv.KubeClient = newDeleteFailingKubeClient()

	mockTestEnv.DeleteTestPods(mockTestSuite.TestManifests)

	stream := mockTestEnv.Stream.(mockStream)
	if len(stream.messages) == 1 {
		t.Errorf("Expected 1 error, got none: %v", stream.messages)
	}
}

type MockTestingEnvironment struct {
	*Environment
}

func newMockTestingEnvironment() *MockTestingEnvironment {
	tEnv := mockTillerEnvironment()

	return &MockTestingEnvironment{
		Environment: &Environment{
			Namespace:  "default",
			KubeClient: tEnv.KubeClient,
			Timeout:    5,
			Stream:     mockStream{},
		},
	}
}

func (mte MockTestingEnvironment) streamRunning(name string) error       { return nil }
func (mte MockTestingEnvironment) streamError(info string) error         { return nil }
func (mte MockTestingEnvironment) streamFailed(name string) error        { return nil }
func (mte MockTestingEnvironment) streamSuccess(name string) error       { return nil }
func (mte MockTestingEnvironment) streamUnknown(name, info string) error { return nil }
func (mte MockTestingEnvironment) streamMessage(msg string) error        { return nil }

func newGetFailingKubeClient() *getFailingKubeClient {
	return &getFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

type getFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func (p *getFailingKubeClient) Get(ns string, r io.Reader) (string, error) {
	return "", errors.New("Get failed")
}

func newDeleteFailingKubeClient() *deleteFailingKubeClient {
	return &deleteFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

type deleteFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func (p *deleteFailingKubeClient) Delete(ns string, r io.Reader) error {
	return errors.New("In the end, they did not find Nemo.")
}
