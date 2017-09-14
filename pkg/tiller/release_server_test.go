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

package tiller

import (
	"errors"
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller/environment"
)

const notesText = "my notes here"

var manifestWithHook = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete
data:
  name: value`

var manifestWithTestHook = `
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

var manifestWithKeep = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm-keep
  annotations:
    "helm.sh/resource-policy": keep
data:
  name: value
`

var manifestWithUpgradeHooks = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-upgrade,pre-upgrade
data:
  name: value`

var manifestWithRollbackHooks = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-rollback,pre-rollback
data:
  name: value
`

func rsFixture() *ReleaseServer {
	clientset := fake.NewSimpleClientset()
	return &ReleaseServer{
		ReleaseModule: &LocalReleaseModule{
			clientset: clientset,
		},
		env:       MockEnvironment(),
		clientset: clientset,
		Log:       func(_ string, _ ...interface{}) {},
	}
}

// chartStub creates a fully stubbed out chart.
func chartStub() *chart.Chart {
	return &chart.Chart{
		// TODO: This should be more complete.
		Metadata: &chart.Metadata{
			Name: "hello",
		},
		// This adds basic templates, partials, and hooks.
		Templates: []*chart.Template{
			{Name: "templates/hello", Data: []byte("hello: world")},
			{Name: "templates/goodbye", Data: []byte("goodbye: world")},
			{Name: "templates/empty", Data: []byte("")},
			{Name: "templates/with-partials", Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
			{Name: "templates/hooks", Data: []byte(manifestWithHook)},
		},
	}
}

// releaseStub creates a release stub, complete with the chartStub as its chart.
func releaseStub() *release.Release {
	return namedReleaseStub("angry-panda", release.Status_DEPLOYED)
}

func namedReleaseStub(name string, status release.Status_Code) *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: status},
			Description:   "Named Release Stub",
		},
		Chart:   chartStub(),
		Config:  &chart.Config{Raw: `name: value`},
		Version: 1,
		Hooks: []*release.Hook{
			{
				Name:     "test-cm",
				Kind:     "ConfigMap",
				Path:     "test-cm",
				Manifest: manifestWithHook,
				Events: []release.Hook_Event{
					release.Hook_POST_INSTALL,
					release.Hook_PRE_DELETE,
				},
			},
			{
				Name:     "finding-nemo",
				Kind:     "Pod",
				Path:     "finding-nemo",
				Manifest: manifestWithTestHook,
				Events: []release.Hook_Event{
					release.Hook_RELEASE_TEST_SUCCESS,
				},
			},
		},
	}
}

func upgradeReleaseVersion(rel *release.Release) *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}

	rel.Info.Status.Code = release.Status_SUPERSEDED
	return &release.Release{
		Name: rel.Name,
		Info: &release.Info{
			FirstDeployed: rel.Info.FirstDeployed,
			LastDeployed:  &date,
			Status:        &release.Status{Code: release.Status_DEPLOYED},
		},
		Chart:   rel.Chart,
		Config:  rel.Config,
		Version: rel.Version + 1,
	}
}

func TestValidName(t *testing.T) {
	for name, valid := range map[string]error{
		"nina pinta santa-maria": errInvalidName,
		"nina-pinta-santa-maria": nil,
		"-nina":                  errInvalidName,
		"pinta-":                 errInvalidName,
		"santa-maria":            nil,
		"niña":                   errInvalidName,
		"...":                    errInvalidName,
		"pinta...":               errInvalidName,
		"santa...maria":          nil,
		"":                       errMissingRelease,
		" ":                      errInvalidName,
		".nina.":                 errInvalidName,
		"nina.pinta":             nil,
		"abcdefghi-abcdefghi-abcdefghi-abcdefghi-abcdefghi-abcd": errInvalidName,
	} {
		if valid != validateReleaseName(name) {
			t.Errorf("Expected %q to be %t", name, valid)
		}
	}
}

func TestGetVersionSet(t *testing.T) {
	rs := rsFixture()
	vs, err := GetVersionSet(rs.clientset.Discovery())
	if err != nil {
		t.Error(err)
	}
	if !vs.Has("v1") {
		t.Errorf("Expected supported versions to at least include v1.")
	}
	if vs.Has("nosuchversion/v1") {
		t.Error("Non-existent version is reported found.")
	}
}

func TestUniqName(t *testing.T) {
	rs := rsFixture()

	rel1 := releaseStub()
	rel2 := releaseStub()
	rel2.Name = "happy-panda"
	rel2.Info.Status.Code = release.Status_DELETED

	rs.env.Releases.Create(rel1)
	rs.env.Releases.Create(rel2)

	tests := []struct {
		name   string
		expect string
		reuse  bool
		err    bool
	}{
		{"first", "first", false, false},
		{"", "[a-z]+-[a-z]+", false, false},
		{"angry-panda", "", false, true},
		{"happy-panda", "", false, true},
		{"happy-panda", "happy-panda", true, false},
		{"hungry-hungry-hungry-hungry-hungry-hungry-hungry-hungry-hippos", "", true, true}, // Exceeds max name length
	}

	for _, tt := range tests {
		u, err := rs.uniqName(tt.name, tt.reuse)
		if err != nil {
			if tt.err {
				continue
			}
			t.Fatal(err)
		}
		if tt.err {
			t.Errorf("Expected an error for %q", tt.name)
		}
		if match, err := regexp.MatchString(tt.expect, u); err != nil {
			t.Fatal(err)
		} else if !match {
			t.Errorf("Expected %q to match %q", u, tt.expect)
		}
	}
}

func releaseWithKeepStub(rlsName string) *release.Release {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "bunnychart",
		},
		Templates: []*chart.Template{
			{Name: "templates/configmap", Data: []byte(manifestWithKeep)},
		},
	}

	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	return &release.Release{
		Name: rlsName,
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: release.Status_DEPLOYED},
		},
		Chart:    ch,
		Config:   &chart.Config{Raw: `name: value`},
		Version:  1,
		Manifest: manifestWithKeep,
	}
}

func MockEnvironment() *environment.Environment {
	e := environment.New()
	e.Releases = storage.Init(driver.NewMemory())
	e.KubeClient = &environment.PrintingKubeClient{Out: os.Stdout}
	return e
}

func newUpdateFailingKubeClient() *updateFailingKubeClient {
	return &updateFailingKubeClient{
		PrintingKubeClient: environment.PrintingKubeClient{Out: os.Stdout},
	}

}

type updateFailingKubeClient struct {
	environment.PrintingKubeClient
}

func (u *updateFailingKubeClient) Update(namespace string, originalReader, modifiedReader io.Reader, force bool, recreate bool, timeout int64, shouldWait bool) error {
	return errors.New("Failed update in kube client")
}

func newHookFailingKubeClient() *hookFailingKubeClient {
	return &hookFailingKubeClient{
		PrintingKubeClient: environment.PrintingKubeClient{Out: os.Stdout},
	}
}

type hookFailingKubeClient struct {
	environment.PrintingKubeClient
}

func (h *hookFailingKubeClient) WatchUntilReady(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	return errors.New("Failed watch")
}

type mockListServer struct {
	val *services.ListReleasesResponse
}

func (l *mockListServer) Send(res *services.ListReleasesResponse) error {
	l.val = res
	return nil
}

func (l *mockListServer) Context() context.Context       { return helm.NewContext() }
func (l *mockListServer) SendMsg(v interface{}) error    { return nil }
func (l *mockListServer) RecvMsg(v interface{}) error    { return nil }
func (l *mockListServer) SendHeader(m metadata.MD) error { return nil }
func (l *mockListServer) SetTrailer(m metadata.MD)       {}
func (l *mockListServer) SetHeader(m metadata.MD) error  { return nil }

type mockRunReleaseTestServer struct {
	stream grpc.ServerStream
}

func (rs mockRunReleaseTestServer) Send(m *services.TestReleaseResponse) error {
	return nil
}
func (rs mockRunReleaseTestServer) SetHeader(m metadata.MD) error  { return nil }
func (rs mockRunReleaseTestServer) SendHeader(m metadata.MD) error { return nil }
func (rs mockRunReleaseTestServer) SetTrailer(m metadata.MD)       {}
func (rs mockRunReleaseTestServer) SendMsg(v interface{}) error    { return nil }
func (rs mockRunReleaseTestServer) RecvMsg(v interface{}) error    { return nil }
func (rs mockRunReleaseTestServer) Context() context.Context       { return helm.NewContext() }
