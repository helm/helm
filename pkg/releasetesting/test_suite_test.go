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
	"k8s.io/kubernetes/pkg/api"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	tillerEnv "k8s.io/helm/pkg/tiller/environment"
)

func TestNewTestSuite(t *testing.T) {
	rel := releaseStub()

	_, err := NewTestSuite(rel)
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestRun(t *testing.T) {

	ts := testSuiteFixture()
	if err := ts.Run(testEnvFixture()); err != nil {
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

	if result.Name != "finding-nemo" {
		t.Errorf("Expected test name to be finding-nemo. Got: %v", result.Name)
	}

	if result.Status != release.TestRun_SUCCESS {
		t.Errorf("Expected test result to be successful, got: %v", result.Status)
	}

}

func TestGetTestPodStatus(t *testing.T) {
	ts := testSuiteFixture()

	status, err := ts.getTestPodStatus(testFixture(), testEnvFixture())
	if err != nil {
		t.Errorf("Expected getTestPodStatus not to return err, Got: %s", err)
	}

	if status != api.PodSucceeded {
		t.Errorf("Expected pod status to be succeeded, Got: %s ", status)
	}

}

func TestExtractTestManifestsFromHooks(t *testing.T) {
	rel := releaseStub()
	testManifests, err := extractTestManifestsFromHooks(rel.Hooks, rel.Name)
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
			{Name: "templates/hooks", Data: []byte(manifestWithTestHook)},
		},
	}
}

var manifestWithTestHook = `
apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`
var manifestWithInstallHooks = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete
data:
  name: value
`

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
				Manifest: manifestWithTestHook,
				Events: []release.Hook_Event{
					release.Hook_RELEASE_TEST,
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
		manifest: manifestWithTestHook,
		result:   &release.TestRun{},
	}
}

func testSuiteFixture() *TestSuite {
	testManifests := []string{manifestWithTestHook}
	testResults := []*release.TestRun{}
	ts := &TestSuite{
		TestManifests: testManifests,
		Results:       testResults,
	}

	return ts
}

func testEnvFixture() *Environment {
	tillerEnv := mockTillerEnvironment()

	return &Environment{
		Namespace:  "default",
		KubeClient: tillerEnv.KubeClient,
		Timeout:    5,
		Stream:     mockStream{},
	}
}

func mockTillerEnvironment() *tillerEnv.Environment {
	e := tillerEnv.New()
	e.Releases = storage.Init(driver.NewMemory())
	e.KubeClient = newPodSucceededKubeClient()
	return e
}

type mockStream struct {
	stream grpc.ServerStream
}

func (rs mockStream) Send(m *services.TestReleaseResponse) error {
	return nil
}
func (rs mockStream) SetHeader(m metadata.MD) error  { return nil }
func (rs mockStream) SendHeader(m metadata.MD) error { return nil }
func (rs mockStream) SetTrailer(m metadata.MD)       {}
func (rs mockStream) SendMsg(v interface{}) error    { return nil }
func (rs mockStream) RecvMsg(v interface{}) error    { return nil }
func (rs mockStream) Context() context.Context       { return helm.NewContext() }

func newPodSucceededKubeClient() *podSucceededKubeClient {
	return &podSucceededKubeClient{
		PrintingKubeClient: tillerEnv.PrintingKubeClient{Out: os.Stdout},
	}
}

type podSucceededKubeClient struct {
	tillerEnv.PrintingKubeClient
}

func (p *podSucceededKubeClient) WaitAndGetCompletedPodPhase(ns string, r io.Reader, timeout time.Duration) (api.PodPhase, error) {
	return api.PodSucceeded, nil
}
