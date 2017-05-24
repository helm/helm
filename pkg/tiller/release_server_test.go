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
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
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
	"k8s.io/helm/pkg/version"
)

const notesText = "my notes here"

var manifestWithHook = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete
data:
  name: value
`

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
  name: value
`

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
	for name, valid := range map[string]bool{
		"nina pinta santa-maria": false,
		"nina-pinta-santa-maria": true,
		"-nina":                  false,
		"pinta-":                 false,
		"santa-maria":            true,
		"niña":                   false,
		"...":                    false,
		"pinta...":               false,
		"santa...maria":          true,
		"":                       false,
		" ":                      false,
		".nina.":                 false,
		"nina.pinta":             true,
	} {
		if valid != ValidName.MatchString(name) {
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

func TestInstallRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
			},
		},
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Release.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Release.Namespace)
	}

	rel, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	if rel.Hooks[0].Events[0] != release.Hook_POST_INSTALL {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.Hook_PRE_DELETE {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Release.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res.Release)
	}

	if len(rel.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if rel.Info.Description != "Install complete" {
		t.Errorf("unexpected description: %s", rel.Info.Description)
	}
}

func TestInstallRelease_WithNotes(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
				{Name: "templates/NOTES.txt", Data: []byte(notesText)},
			},
		},
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Release.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Release.Namespace)
	}

	rel, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	if rel.Info.Status.Notes != notesText {
		t.Fatalf("Expected '%s', got '%s'", notesText, rel.Info.Status.Notes)
	}

	if rel.Hooks[0].Events[0] != release.Hook_POST_INSTALL {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.Hook_PRE_DELETE {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Release.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res.Release)
	}

	if len(rel.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if rel.Info.Description != "Install complete" {
		t.Errorf("unexpected description: %s", rel.Info.Description)
	}
}

func TestInstallRelease_WithNotesRendered(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
				{Name: "templates/NOTES.txt", Data: []byte(notesText + " {{.Release.Name}}")},
			},
		},
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Release.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Release.Namespace)
	}

	rel, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	expectedNotes := fmt.Sprintf("%s %s", notesText, res.Release.Name)
	if rel.Info.Status.Notes != expectedNotes {
		t.Fatalf("Expected '%s', got '%s'", expectedNotes, rel.Info.Status.Notes)
	}

	if rel.Hooks[0].Events[0] != release.Hook_POST_INSTALL {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.Hook_PRE_DELETE {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Release.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res.Release)
	}

	if len(rel.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if rel.Info.Description != "Install complete" {
		t.Errorf("unexpected description: %s", rel.Info.Description)
	}
}

func TestInstallRelease_TillerVersion(t *testing.T) {
	version.Version = "2.2.0"
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello", TillerVersion: ">=2.2.0"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
			},
		},
	}
	_, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Expected valid range. Got %q", err)
	}
}

func TestInstallRelease_WrongTillerVersion(t *testing.T) {
	version.Version = "2.2.0"
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello", TillerVersion: "<2.0.0"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
			},
		},
	}
	_, err := rs.InstallRelease(c, req)
	if err == nil {
		t.Fatalf("Expected to fail because of wrong version")
	}

	expect := "Chart incompatible with Tiller"
	if !strings.Contains(err.Error(), expect) {
		t.Errorf("Expected %q to contain %q", err.Error(), expect)
	}
}

func TestInstallRelease_WithChartAndDependencyNotes(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	// TODO: Refactor this into a mock.
	req := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
				{Name: "templates/NOTES.txt", Data: []byte(notesText)},
			},
			Dependencies: []*chart.Chart{
				{
					Metadata: &chart.Metadata{Name: "hello"},
					Templates: []*chart.Template{
						{Name: "templates/hello", Data: []byte("hello: world")},
						{Name: "templates/hooks", Data: []byte(manifestWithHook)},
						{Name: "templates/NOTES.txt", Data: []byte(notesText + " child")},
					},
				},
			},
		},
	}

	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	rel, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	t.Logf("rel: %v", rel)

	if rel.Info.Status.Notes != notesText {
		t.Fatalf("Expected '%s', got '%s'", notesText, rel.Info.Status.Notes)
	}

	if rel.Info.Description != "Install complete" {
		t.Errorf("unexpected description: %s", rel.Info.Description)
	}
}

func TestInstallRelease_DryRun(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	req := &services.InstallReleaseRequest{
		Chart:  chartStub(),
		DryRun: true,
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	if !strings.Contains(res.Release.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", res.Release.Manifest)
	}

	if !strings.Contains(res.Release.Manifest, "---\n# Source: hello/templates/goodbye\ngoodbye: world") {
		t.Errorf("unexpected output: %s", res.Release.Manifest)
	}

	if !strings.Contains(res.Release.Manifest, "hello: Earth") {
		t.Errorf("Should contain partial content. %s", res.Release.Manifest)
	}

	if strings.Contains(res.Release.Manifest, "hello: {{ template \"_planet\" . }}") {
		t.Errorf("Should not contain partial templates itself. %s", res.Release.Manifest)
	}

	if strings.Contains(res.Release.Manifest, "empty") {
		t.Errorf("Should not contain template data for an empty file. %s", res.Release.Manifest)
	}

	if _, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version); err == nil {
		t.Errorf("Expected no stored release.")
	}

	if l := len(res.Release.Hooks); l != 1 {
		t.Fatalf("Expected 1 hook, got %d", l)
	}

	if res.Release.Hooks[0].LastRun != nil {
		t.Error("Expected hook to not be marked as run.")
	}

	if res.Release.Info.Description != "Dry run complete" {
		t.Errorf("unexpected description: %s", res.Release.Info.Description)
	}
}

func TestInstallRelease_NoHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.InstallReleaseRequest{
		Chart:        chartStub(),
		DisableHooks: true,
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	if hl := res.Release.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())
	rs.env.KubeClient = newHookFailingKubeClient()

	req := &services.InstallReleaseRequest{
		Chart: chartStub(),
	}
	res, err := rs.InstallRelease(c, req)
	if err == nil {
		t.Error("Expected failed install")
	}

	if hl := res.Release.Info.Status.Code; hl != release.Status_FAILED {
		t.Errorf("Expected FAILED release. Got %d", hl)
	}
}

func TestInstallRelease_ReuseName(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rel.Info.Status.Code = release.Status_DELETED
	rs.env.Releases.Create(rel)

	req := &services.InstallReleaseRequest{
		Chart:     chartStub(),
		ReuseName: true,
		Name:      rel.Name,
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	if res.Release.Name != rel.Name {
		t.Errorf("expected %q, got %q", rel.Name, res.Release.Name)
	}

	getreq := &services.GetReleaseStatusRequest{Name: rel.Name, Version: 0}
	getres, err := rs.GetReleaseStatus(c, getreq)
	if err != nil {
		t.Errorf("Failed to retrieve release: %s", err)
	}
	if getres.Info.Status.Code != release.Status_DEPLOYED {
		t.Errorf("Release status is %q", getres.Info.Status.Code)
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

func (u *updateFailingKubeClient) Update(namespace string, originalReader, modifiedReader io.Reader, recreate bool, timeout int64, shouldWait bool) error {
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
