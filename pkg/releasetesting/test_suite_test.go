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
	"io"
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"k8s.io/kubernetes/pkg/apis/core"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	tillerEnv "k8s.io/helm/pkg/tiller/environment"
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

func TestNewTestSuite(t *testing.T) {
	rel := releaseStub()

	_, err := NewTestSuite(rel)
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestRun(t *testing.T) {

	testManifests := []string{manifestWithTestSuccessHook, manifestWithTestFailureHook}
	ts := testSuiteFixture(testManifests)
	if err := ts.Run(testEnvFixture()); err != nil {
		t.Errorf("%s", err)
	}

	if ts.StartedAt == nil {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}

	if ts.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}

	if len(ts.Results) != 2 {
		t.Errorf("Expected 2 test result. Got %v", len(ts.Results))
	}

	result := ts.Results[0]
	if result.StartedAt == nil {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result.StartedAt)
	}

	if result.CompletedAt == nil {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result.CompletedAt)
	}

	if result.Name != "finding-nemo" {
		t.Errorf("Expected test name to be finding-nemo. Got: %v", result.Name)
	}

	if result.Status != release.TestRun_SUCCESS {
		t.Errorf("Expected test result to be successful, got: %v", result.Status)
	}

	result2 := ts.Results[1]
	if result2.StartedAt == nil {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result2.StartedAt)
	}

	if result2.CompletedAt == nil {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result2.CompletedAt)
	}

	if result2.Name != "gold-rush" {
		t.Errorf("Expected test name to be gold-rush, Got: %v", result2.Name)
	}

	if result2.Status != release.TestRun_FAILURE {
		t.Errorf("Expected test result to be successful, got: %v", result2.Status)
	}

}

func TestRunEmptyTestSuite(t *testing.T) {
	ts := testSuiteFixture([]string{})
	mockTestEnv := testEnvFixture()
	if err := ts.Run(mockTestEnv); err != nil {
		t.Errorf("%s", err)
	}

	if ts.StartedAt == nil {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}

	if ts.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}

	if len(ts.Results) != 0 {
		t.Errorf("Expected 0 test result. Got %v", len(ts.Results))
	}

	stream := mockTestEnv.Stream.(*mockStream)
	if len(stream.messages) == 0 {
		t.Errorf("Expected at least one message, Got: %v", len(stream.messages))
	} else {
		msg := stream.messages[0].Msg
		if msg != "No Tests Found" {
			t.Errorf("Expected message 'No Tests Found', Got: %v", msg)
		}
	}

}

func TestRunSuccessWithTestFailureHook(t *testing.T) {
	ts := testSuiteFixture([]string{manifestWithTestFailureHook})
	env := testEnvFixture()
	env.KubeClient = newPodFailedKubeClient()
	if err := ts.Run(env); err != nil {
		t.Errorf("%s", err)
	}

	if ts.StartedAt == nil {
		t.Errorf("Expected StartedAt to not be nil. Got: %v", ts.StartedAt)
	}

	if ts.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to not be nil. Got: %v", ts.CompletedAt)
	}

	if len(ts.Results) != 1 {
		t.Errorf("Expected 1 test result. Got %v", len(ts.Results))
	}

	result := ts.Results[0]
	if result.StartedAt == nil {
		t.Errorf("Expected test StartedAt to not be nil. Got: %v", result.StartedAt)
	}

	if result.CompletedAt == nil {
		t.Errorf("Expected test CompletedAt to not be nil. Got: %v", result.CompletedAt)
	}

	if result.Name != "gold-rush" {
		t.Errorf("Expected test name to be gold-rush, Got: %v", result.Name)
	}

	if result.Status != release.TestRun_SUCCESS {
		t.Errorf("Expected test result to be successful, got: %v", result.Status)
	}
}

func TestExtractTestManifestsFromHooks(t *testing.T) {
	rel := releaseStub()
	testManifests, err := extractTestManifestsFromHooks(rel.Hooks)
	if err != nil {
		t.Errorf("Expected no error, Got: %s", err)
	}

	if len(testManifests) != 1 {
		t.Errorf("Expected 1 test manifest, Got: %v", len(testManifests))
	}
}

func chartStub() *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "nemo",
		},
		Templates: []*chart.Template{
			{Name: "templates/hello", Data: []byte("hello: world")},
			{Name: "templates/hooks", Data: []byte(manifestWithTestSuccessHook)},
		},
	}
}

func releaseStub() *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	return &release.Release{
		Name: "lost-fish",
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: release.Status_DEPLOYED},
			Description:   "a release stub",
		},
		Chart:   chartStub(),
		Config:  &chart.Config{Raw: `name: value`},
		Version: 1,
		Hooks: []*release.Hook{
			{
				Name:     "finding-nemo",
				Kind:     "Pod",
				Path:     "finding-nemo",
				Manifest: manifestWithTestSuccessHook,
				Events: []release.Hook_Event{
					release.Hook_RELEASE_TEST_SUCCESS,
				},
			},
			{
				Name:     "test-cm",
				Kind:     "ConfigMap",
				Path:     "test-cm",
				Manifest: manifestWithInstallHooks,
				Events: []release.Hook_Event{
					release.Hook_POST_INSTALL,
					release.Hook_PRE_DELETE,
				},
			},
		},
	}
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
	return newMockTestingEnvironment().Environment
}

func mockTillerEnvironment() *tillerEnv.Environment {
	e := tillerEnv.New()
	e.Releases = storage.Init(driver.NewMemory())
	e.KubeClient = newPodSucceededKubeClient()
	return e
}

type mockStream struct {
	stream   grpc.ServerStream
	messages []*services.TestReleaseResponse
}

func (rs *mockStream) Send(m *services.TestReleaseResponse) error {
	rs.messages = append(rs.messages, m)
	return nil
}

func (rs mockStream) SetHeader(m metadata.MD) error  { return nil }
func (rs mockStream) SendHeader(m metadata.MD) error { return nil }
func (rs mockStream) SetTrailer(m metadata.MD)       {}
func (rs mockStream) SendMsg(v interface{}) error    { return nil }
func (rs mockStream) RecvMsg(v interface{}) error    { return nil }
func (rs mockStream) Context() context.Context       { return helm.NewContext() }

type podSucceededKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newPodSucceededKubeClient() *podSucceededKubeClient {
	return &podSucceededKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

func (p *podSucceededKubeClient) WaitAndGetCompletedPodPhase(ns string, r io.Reader, timeout time.Duration) (core.PodPhase, error) {
	return core.PodSucceeded, nil
}

type podFailedKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func newPodFailedKubeClient() *podFailedKubeClient {
	return &podFailedKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

func (p *podFailedKubeClient) WaitAndGetCompletedPodPhase(ns string, r io.Reader, timeout time.Duration) (core.PodPhase, error) {
	return core.PodFailed, nil
}
