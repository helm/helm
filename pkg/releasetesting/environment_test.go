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
	"io/ioutil"
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

	for _, testManifest := range mockTestSuite.TestManifests {
		if _, err := mockTestEnv.KubeClient.Get(mockTestEnv.Namespace, bytes.NewBufferString(testManifest)); err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestStreamMessage(t *testing.T) {
	tEnv := mockTillerEnvironment()

	ch := make(chan *services.TestReleaseResponse, 1)
	defer close(ch)

	mockTestEnv := &Environment{
		Namespace:  "default",
		KubeClient: tEnv.KubeClient,
		Timeout:    1,
		Mesages:    ch,
	}

	expectedMessage := "testing streamMessage"
	expectedStatus := release.TestRun_SUCCESS
	err := mockTestEnv.streamMessage(expectedMessage, expectedStatus)
	if err != nil {
		t.Errorf("Expected no errors, got: %s", err)
	}

	got := <-mockTestEnv.Mesages
	if got.Msg != expectedMessage {
		t.Errorf("Expected message: %s, got: %s", expectedMessage, got.Msg)
	}
	if got.Status != expectedStatus {
		t.Errorf("Expected status: %v, got: %v", expectedStatus, got.Status)
	}
}

func newMockTestingEnvironment() *Environment {
	return &Environment{
		Namespace:  "default",
		KubeClient: mockTillerEnvironment().KubeClient,
		Timeout:    1,
	}
}

type getFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newGetFailingKubeClient() *getFailingKubeClient {
	return &getFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: ioutil.Discard},
	}
}

func (p *getFailingKubeClient) Get(ns string, r io.Reader) (string, error) {
	return "", errors.New("in the end, they did not find Nemo")
}

type createFailingKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newCreateFailingKubeClient() *createFailingKubeClient {
	return &createFailingKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: ioutil.Discard},
	}
}

func (p *createFailingKubeClient) Create(ns string, r io.Reader, t int64, shouldWait bool) error {
	return errors.New("We ran out of budget and couldn't create finding-nemo")
}
