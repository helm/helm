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
	"testing"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/release"
)

func TestCreateTestPodSuccess(t *testing.T) {
	env := testEnvFixture()
	test := testFixture()

	if err := env.createTestPod(test); err != nil {
		t.Errorf("Expected no error, got an error: %s", err)
	}
}

func TestCreateTestPodFailure(t *testing.T) {
	env := testEnvFixture()
	env.KubeClient = &mockKubeClient{
		err: errors.New("We ran out of budget and couldn't create finding-nemo"),
	}
	test := testFixture()

	if err := env.createTestPod(test); err == nil {
		t.Errorf("Expected error, got no error")
	}
	if test.result.Info == "" {
		t.Errorf("Expected error to be saved in test result info but found empty string")
	}
	if test.result.Status != release.TestRunFailure {
		t.Errorf("Expected test result status to be failure but got: %v", test.result.Status)
	}
}

func TestStreamMessage(t *testing.T) {
	env := testEnvFixture()
	defer close(env.Messages)

	expectedMessage := "testing streamMessage"
	expectedStatus := release.TestRunSuccess
	if err := env.streamMessage(expectedMessage, expectedStatus); err != nil {
		t.Errorf("Expected no errors, got: %s", err)
	}

	got := <-env.Messages
	if got.Msg != expectedMessage {
		t.Errorf("Expected message: %s, got: %s", expectedMessage, got.Msg)
	}
	if got.Status != expectedStatus {
		t.Errorf("Expected status: %v, got: %v", expectedStatus, got.Status)
	}
}
