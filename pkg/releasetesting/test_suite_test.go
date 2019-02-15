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
	"io"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/tiller/environment"
)

const manifestWithTestSuccessHook = `
apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test-success
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`

const manifestWithTestFailureHook = `
apiVersion: v1
kind: Pod
metadata:
  name: gold-rush,
  annotations:
    "helm.sh/hook": test-failure
spec:
  containers:
  - name: gold-finding-test
    image: fake-gold-finding-image
    cmd: fake-gold-finding-command
`
const manifestWithInstallHooks = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete
data:
  name: value
`

func TestRun(t *testing.T) {
	testManifests := []string{manifestWithTestSuccessHook, manifestWithTestFailureHook}
	ts := testSuiteFixture(testManifests)
	env := testEnvFixture()

	go func() {
		defer close(env.Mesages)
		if err := ts.Run(env); err != nil {
			t.Error(err)
		}
	}()

	for i := 0; i <= 4; i++ {
		<-env.Mesages
	}
	if _, ok := <-env.Mesages; ok {
		t.Errorf("Expected 4 messages streamed")
	}

	if ts.StartedAt.IsZero() {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}
	if ts.CompletedAt.IsZero() {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}
	if len(ts.Results) != 2 {
		t.Errorf("Expected 2 test result. Got %v", len(ts.Results))
	}

	result := ts.Results[0]
	if result.StartedAt.IsZero() {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result.StartedAt)
	}
	if result.CompletedAt.IsZero() {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result.CompletedAt)
	}
	if result.Name != "finding-nemo" {
		t.Errorf("Expected test name to be finding-nemo. Got: %v", result.Name)
	}
	if result.Status != release.TestRunSuccess {
		t.Errorf("Expected test result to be successful, got: %v", result.Status)
	}
	result2 := ts.Results[1]
	if result2.StartedAt.IsZero() {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result2.StartedAt)
	}
	if result2.CompletedAt.IsZero() {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result2.CompletedAt)
	}
	if result2.Name != "gold-rush" {
		t.Errorf("Expected test name to be gold-rush, Got: %v", result2.Name)
	}
	if result2.Status != release.TestRunFailure {
		t.Errorf("Expected test result to be successful, got: %v", result2.Status)
	}
}

func TestRunEmptyTestSuite(t *testing.T) {
	ts := testSuiteFixture([]string{})
	env := testEnvFixture()

	go func() {
		defer close(env.Mesages)
		if err := ts.Run(env); err != nil {
			t.Error(err)
		}
	}()

	msg := <-env.Mesages
	if msg.Msg != "No Tests Found" {
		t.Errorf("Expected message 'No Tests Found', Got: %v", msg.Msg)
	}

	for range env.Mesages {
	}

	if ts.StartedAt.IsZero() {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}
	if ts.CompletedAt.IsZero() {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}
	if len(ts.Results) != 0 {
		t.Errorf("Expected 0 test result. Got %v", len(ts.Results))
	}
}

func TestRunSuccessWithTestFailureHook(t *testing.T) {
	ts := testSuiteFixture([]string{manifestWithTestFailureHook})
	env := testEnvFixture()
	env.KubeClient = &mockKubeClient{podFail: true}

	go func() {
		defer close(env.Mesages)
		if err := ts.Run(env); err != nil {
			t.Error(err)
		}
	}()

	for i := 0; i <= 4; i++ {
		<-env.Mesages
	}
	if _, ok := <-env.Mesages; ok {
		t.Errorf("Expected 4 messages streamed")
	}

	if ts.StartedAt.IsZero() {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}
	if ts.CompletedAt.IsZero() {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}
	if len(ts.Results) != 1 {
		t.Errorf("Expected 1 test result. Got %v", len(ts.Results))
	}

	result := ts.Results[0]
	if result.StartedAt.IsZero() {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result.StartedAt)
	}
	if result.CompletedAt.IsZero() {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result.CompletedAt)
	}
	if result.Name != "gold-rush" {
		t.Errorf("Expected test name to be gold-rush, Got: %v", result.Name)
	}
	if result.Status != release.TestRunSuccess {
		t.Errorf("Expected test result to be successful, got: %v", result.Status)
	}
}

func TestExtractTestManifestsFromHooks(t *testing.T) {
	testManifests := extractTestManifestsFromHooks(hooksStub)

	if len(testManifests) != 1 {
		t.Errorf("Expected 1 test manifest, Got: %v", len(testManifests))
	}
}

var hooksStub = []*release.Hook{
	{
		Manifest: manifestWithTestSuccessHook,
		Events: []release.HookEvent{
			release.HookReleaseTestSuccess,
		},
	},
	{
		Manifest: manifestWithInstallHooks,
		Events: []release.HookEvent{
			release.HookPostInstall,
		},
	},
}

func testFixture() *test {
	return &test{
		manifest: manifestWithTestSuccessHook,
		result:   &release.TestRun{},
	}
}

func testSuiteFixture(testManifests []string) *TestSuite {
	testResults := []*release.TestRun{}
	ts := &TestSuite{
		TestManifests: testManifests,
		Results:       testResults,
	}
	return ts
}

func testEnvFixture() *Environment {
	return &Environment{
		Namespace:  "default",
		KubeClient: &mockKubeClient{},
		Timeout:    1,
		Mesages:    make(chan *hapi.TestReleaseResponse, 1),
	}
}

type mockKubeClient struct {
	environment.KubeClient
	podFail bool
	err     error
}

func (c *mockKubeClient) WaitAndGetCompletedPodPhase(_ string, _ io.Reader, _ time.Duration) (v1.PodPhase, error) {
	if c.podFail {
		return v1.PodFailed, nil
	}
	return v1.PodSucceeded, nil
}
func (c *mockKubeClient) Get(_ string, _ io.Reader) (string, error) {
	return "", nil
}
func (c *mockKubeClient) Create(_ string, _ io.Reader, _ int64, _ bool) error {
	return c.err
}
func (c *mockKubeClient) Delete(_ string, _ io.Reader) error {
	return nil
}
