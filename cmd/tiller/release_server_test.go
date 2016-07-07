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

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	"k8s.io/helm/cmd/tiller/environment"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/timeconv"
)

var manifestWithHook = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete
data:
  name: value
`

func rsFixture() *releaseServer {
	return &releaseServer{
		env: mockEnvironment(),
	}
}

func releaseMock() *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	return &release.Release{
		Name: "angry-panda",
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: release.Status_DEPLOYED},
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "foo",
				Version: "0.1.0-beta.1",
			},
			Templates: []*chart.Template{
				{Name: "foo.tpl", Data: []byte("Hello")},
			},
		},
		Config: &chart.Config{Raw: `name = "value"`},
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
		},
	}
}

func TestInstallRelease(t *testing.T) {
	c := context.Background()
	rs := rsFixture()

	req := &services.InstallReleaseRequest{
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "hello", Data: []byte("hello: world")},
				{Name: "hooks", Data: []byte(manifestWithHook)},
			},
		},
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	rel, err := rs.env.Releases.Read(res.Release.Name)
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

	if !strings.Contains(rel.Manifest, "---\n# Source: hello/hello\nhello: world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}
}

func TestInstallReleaseDryRun(t *testing.T) {
	c := context.Background()
	rs := rsFixture()

	req := &services.InstallReleaseRequest{
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "hello", Data: []byte("hello: world")},
				{Name: "goodbye", Data: []byte("goodbye: world")},
				{Name: "empty", Data: []byte("")},
				{Name: "with-partials", Data: []byte(`hello: {{ template "_planet" . }}`)},
				{Name: "partials/_planet", Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
				{Name: "hooks", Data: []byte(manifestWithHook)},
			},
		},
		DryRun: true,
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	if !strings.Contains(res.Release.Manifest, "---\n# Source: hello/hello\nhello: world") {
		t.Errorf("unexpected output: %s", res.Release.Manifest)
	}

	if !strings.Contains(res.Release.Manifest, "---\n# Source: hello/goodbye\ngoodbye: world") {
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

	if _, err := rs.env.Releases.Read(res.Release.Name); err == nil {
		t.Errorf("Expected no stored release.")
	}

	if l := len(res.Release.Hooks); l != 1 {
		t.Fatalf("Expected 1 hook, got %d", l)
	}

	if res.Release.Hooks[0].LastRun != nil {
		t.Error("Expected hook to not be marked as run.")
	}
}

func TestUninstallRelease(t *testing.T) {
	c := context.Background()
	rs := rsFixture()
	rs.env.Releases.Create(&release.Release{
		Name: "angry-panda",
		Info: &release.Info{
			FirstDeployed: timeconv.Now(),
			Status: &release.Status{
				Code: release.Status_DEPLOYED,
			},
		},
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
		},
	})

	req := &services.UninstallReleaseRequest{
		Name: "angry-panda",
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed uninstall: %s", err)
	}

	if res.Release.Name != "angry-panda" {
		t.Errorf("Expected angry-panda, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Release.Hooks[0].LastRun.Seconds == 0 {
		t.Error("Expected LastRun to be greater than zero.")
	}

	if res.Release.Info.Deleted.Seconds <= 0 {
		t.Errorf("Expected valid UNIX date, got %d", res.Release.Info.Deleted.Seconds)
	}
}

func TestGetReleaseContent(t *testing.T) {
	c := context.Background()
	rs := rsFixture()
	rel := releaseMock()
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseContent(c, &services.GetReleaseContentRequest{Name: rel.Name})
	if err != nil {
		t.Errorf("Error getting release content: %s", err)
	}

	if res.Release.Chart.Metadata.Name != rel.Chart.Metadata.Name {
		t.Errorf("Expected %q, got %q", rel.Chart.Metadata.Name, res.Release.Chart.Metadata.Name)
	}
}

func TestGetReleaseStatus(t *testing.T) {
	c := context.Background()
	rs := rsFixture()
	rel := releaseMock()
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseStatus(c, &services.GetReleaseStatusRequest{Name: rel.Name})
	if err != nil {
		t.Errorf("Error getting release content: %s", err)
	}

	if res.Info.Status.Code != release.Status_DEPLOYED {
		t.Errorf("Expected %d, got %d", release.Status_DEPLOYED, res.Info.Status.Code)
	}
}

func TestListReleases(t *testing.T) {
	rs := rsFixture()
	num := 7
	for i := 0; i < num; i++ {
		rel := releaseMock()
		rel.Name = fmt.Sprintf("rel-%d", i)
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	mrs := &mockListServer{}
	if err := rs.ListReleases(&services.ListReleasesRequest{Offset: "", Limit: 64}, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != num {
		t.Errorf("Expected %d releases, got %d", num, len(mrs.val.Releases))
	}
}

func TestListReleasesSort(t *testing.T) {
	rs := rsFixture()

	// Put them in by reverse order so that the mock doesn't "accidentally"
	// sort.
	num := 7
	for i := num; i > 0; i-- {
		rel := releaseMock()
		rel.Name = fmt.Sprintf("rel-%d", i)
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	limit := 6
	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset: "",
		Limit:  int64(limit),
		SortBy: services.ListSort_NAME,
	}
	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != limit {
		t.Errorf("Expected %d releases, got %d", limit, len(mrs.val.Releases))
	}

	for i := 0; i < limit; i++ {
		n := fmt.Sprintf("rel-%d", i+1)
		if mrs.val.Releases[i].Name != n {
			t.Errorf("Expected %q, got %q", n, mrs.val.Releases[i].Name)
		}
	}
}

func TestListReleasesFilter(t *testing.T) {
	rs := rsFixture()
	names := []string{
		"axon",
		"dendrite",
		"neuron",
		"neuroglia",
		"synapse",
		"nucleus",
		"organelles",
	}
	num := 7
	for i := 0; i < num; i++ {
		rel := releaseMock()
		rel.Name = names[i]
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset: "",
		Limit:  64,
		Filter: "neuro[a-z]+",
		SortBy: services.ListSort_NAME,
	}
	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(mrs.val.Releases))
	}

	if mrs.val.Releases[0].Name != "neuroglia" {
		t.Errorf("Unexpected sort order: %v.", mrs.val.Releases)
	}
	if mrs.val.Releases[1].Name != "neuron" {
		t.Errorf("Unexpected sort order: %v.", mrs.val.Releases)
	}
}

func mockEnvironment() *environment.Environment {
	e := environment.New()
	e.Releases = storage.NewMemory()
	e.KubeClient = &environment.PrintingKubeClient{Out: os.Stdout}
	return e
}

type mockListServer struct {
	val *services.ListReleasesResponse
}

func (l *mockListServer) Send(res *services.ListReleasesResponse) error {
	l.val = res
	return nil
}

func (l *mockListServer) Context() context.Context       { return context.TODO() }
func (l *mockListServer) SendMsg(v interface{}) error    { return nil }
func (l *mockListServer) RecvMsg(v interface{}) error    { return nil }
func (l *mockListServer) SendHeader(m metadata.MD) error { return nil }
func (l *mockListServer) SetTrailer(m metadata.MD)       {}
