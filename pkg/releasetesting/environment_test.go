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

	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	tillerEnv "k8s.io/helm/pkg/tiller/environment"
)

func TestCreateTestPodSuccess(t *testing.T) {
	env := testEnvFixture()
	test := testFixture()

	err := env.createTestPod(test)
	if err != nil {
		t.Errorf("Expected no error, got an error: %s", err)
	}
}

func TestCreateTestPodFailure(t *testing.T) {
	env := testEnvFixture()
	env.KubeClient = newCreateFailingKubeClient()
	test := testFixture()

	err := env.createTestPod(test)
	if err == nil {
		t.Errorf("Expected error, got no error")
	}

	if test.result.Info == "" {
		t.Errorf("Expected error to be saved in test result info but found empty string")
	}

	if test.result.Status != release.TestRun_FAILURE {
		t.Errorf("Expected test result status to be failure but got: %v", test.result.Status)
	}
}

func TestDeleteTestPods(t *testing.T) {
	mockTestSuite := testSuiteFixture([]string{manifestWithTestSuccessHook})
	mockTestEnv := newMockTestingEnvironment()
	mockTestEnv.KubeClient = newGetFailingKubeClient()

	mockTestEnv.DeleteTestPods(mockTestSuite.TestManifests)

	stream := mockTestEnv.Stream.(*mockStream)
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
	mockTestSuite := testSuiteFixture([]string{manifestWithTestSuccessHook})
	mockTestEnv := newMockTestingEnvironment()
	mockTestEnv.KubeClient = newDeleteFailingKubeClient()

	mockTestEnv.DeleteTestPods(mockTestSuite.TestManifests)

	stream := mockTestEnv.Stream.(*mockStream)
	if len(stream.messages) != 1 {
		t.Errorf("Expected 1 error, got: %v", len(stream.messages))
	}
}

func TestStreamMessage(t *testing.T) {
	mockTestEnv := newMockTestingEnvironment()

	expectedMessage := "testing streamMessage"
	expectedStatus := release.TestRun_SUCCESS
	err := mockTestEnv.streamMessage(expectedMessage, expectedStatus)
	if err != nil {
		t.Errorf("Expected no errors, got 1: %s", err)
	}

	stream := mockTestEnv.Stream.(*mockStream)
	if len(stream.messages) != 1 {
		t.Errorf("Expected 1 message, got: %v", len(stream.messages))
	}

	if stream.messages[0].Msg != expectedMessage {
		t.Errorf("Expected message: %s, got: %s", expectedMessage, stream.messages[0])
	}
	if stream.messages[0].Status != expectedStatus {
		t.Errorf("Expected status: %v, got: %v", expectedStatus, stream.messages[0].Status)
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
			Stream:     &mockStream{},
		},
	}
}

func (mte MockTestingEnvironment) streamRunning(name string) error       { return nil }
func (mte MockTestingEnvironment) streamError(info string) error         { return nil }
func (mte MockTestingEnvironment) streamFailed(name string) error        { return nil }
func (mte MockTestingEnvironment) streamSuccess(name string) error       { return nil }
func (mte MockTestingEnvironment) streamUnknown(name, info string) error { return nil }
func (mte MockTestingEnvironment) streamMessage(msg string, status release.TestRun_Status) error {
	mte.Stream.Send(&services.TestReleaseResponse{Msg: msg, Status: status})
	return nil
}

type getFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newGetFailingKubeClient() *getFailingKubeClient {
	return &getFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

func (p *getFailingKubeClient) Get(ns string, r io.Reader) (string, error) {
	return "", errors.New("in the end, they did not find Nemo")
}

type deleteFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newDeleteFailingKubeClient() *deleteFailingKubeClient {
	return &deleteFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

func (p *deleteFailingKubeClient) Delete(ns string, r io.Reader) error {
	return errors.New("delete failed")
}

type createFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newCreateFailingKubeClient() *createFailingKubeClient {
	return &createFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

func (p *createFailingKubeClient) Create(ns string, r io.Reader, t int64, shouldWait bool) error {
	return errors.New("We ran out of budget and couldn't create finding-nemo")
}
