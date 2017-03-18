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
	return &ReleaseServer{
		env:       MockEnvironment(),
		clientset: fake.NewSimpleClientset(),
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
	vs, err := getVersionSet(rs.clientset.Discovery())
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

func TestInstallReleaseWithNotes(t *testing.T) {
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

func TestInstallReleaseWithNotesRendered(t *testing.T) {
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

func TestInstallReleaseWithChartAndDependencyNotes(t *testing.T) {
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

func TestInstallReleaseDryRun(t *testing.T) {
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

func TestInstallReleaseNoHooks(t *testing.T) {
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

func TestInstallReleaseFailedHooks(t *testing.T) {
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

func TestInstallReleaseReuseName(t *testing.T) {
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

func TestUpdateRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
	}
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	if res.Release.Name != rel.Name {
		t.Errorf("Updated release name does not match previous release name. Expected %s, got %s", rel.Name, res.Release.Name)
	}

	if res.Release.Namespace != rel.Namespace {
		t.Errorf("Expected release namespace '%s', got '%s'.", rel.Namespace, res.Release.Namespace)
	}

	updated, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	if len(updated.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(updated.Hooks))
	}
	if updated.Hooks[0].Manifest != manifestWithUpgradeHooks {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	if updated.Hooks[0].Events[0] != release.Hook_POST_UPGRADE {
		t.Errorf("Expected event 0 to be post upgrade")
	}

	if updated.Hooks[0].Events[1] != release.Hook_PRE_UPGRADE {
		t.Errorf("Expected event 0 to be pre upgrade")
	}

	if len(res.Release.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res.Release)
	}

	if res.Release.Config == nil {
		t.Errorf("Got release without config: %#v", res.Release)
	} else if res.Release.Config.Raw != rel.Config.Raw {
		t.Errorf("Expected release values %q, got %q", rel.Config.Raw, res.Release.Config.Raw)
	}

	if len(updated.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(updated.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if res.Release.Version != 2 {
		t.Errorf("Expected release version to be %v, got %v", 2, res.Release.Version)
	}

	edesc := "Upgrade complete"
	if got := res.Release.Info.Description; got != edesc {
		t.Errorf("Expected description %q, got %q", edesc, got)
	}
}
func TestUpdateRelease_ResetValues(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
		ResetValues: true,
	}
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Release.Config != nil && res.Release.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Release.Config.Raw)
	}
}

func TestUpdateRelease_ReuseValues(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
			// Since reuseValues is set, this should get ignored.
			Values: &chart.Config{Raw: "foo: bar\n"},
		},
		Values:      &chart.Config{Raw: "name2: val2"},
		ReuseValues: true,
	}
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been overwritten with the old value.
	expect := "name: value\n"
	if res.Release.Chart.Values != nil && res.Release.Chart.Values.Raw != expect {
		t.Errorf("Expected chart values to be %q, got %q", expect, res.Release.Chart.Values.Raw)
	}
	// This should have the newly-passed overrides.
	expect = "name2: val2"
	if res.Release.Config != nil && res.Release.Config.Raw != expect {
		t.Errorf("Expected request config to be %q, got %q", expect, res.Release.Config.Raw)
	}
}

func TestUpdateRelease_ResetReuseValues(t *testing.T) {
	// This verifies that when both reset and reuse are set, reset wins.
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
		ResetValues: true,
		ReuseValues: true,
	}
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Release.Config != nil && res.Release.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Release.Config.Raw)
	}
}

func TestUpdateReleaseFailure(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	rs.env.KubeClient = newUpdateFailingKubeClient()

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/something", Data: []byte("hello: world")},
			},
		},
	}

	res, err := rs.UpdateRelease(c, req)
	if err == nil {
		t.Error("Expected failed update")
	}

	if updatedStatus := res.Release.Info.Status.Code; updatedStatus != release.Status_FAILED {
		t.Errorf("Expected FAILED release. Got %d", updatedStatus)
	}

	edesc := "Upgrade \"angry-panda\" failed: Failed update in kube client"
	if got := res.Release.Info.Description; got != edesc {
		t.Errorf("Expected description %q, got %q", edesc, got)
	}

	oldRelease, err := rs.env.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status.Code; oldStatus != release.Status_SUPERSEDED {
		t.Errorf("Expected SUPERSEDED status on previous Release version. Got %v", oldStatus)
	}
}

func TestRollbackReleaseFailure(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
	}

	rs.env.KubeClient = newUpdateFailingKubeClient()
	res, err := rs.RollbackRelease(c, req)
	if err == nil {
		t.Error("Expected failed rollback")
	}

	if targetStatus := res.Release.Info.Status.Code; targetStatus != release.Status_FAILED {
		t.Errorf("Expected FAILED release. Got %v", targetStatus)
	}

	oldRelease, err := rs.env.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status.Code; oldStatus != release.Status_SUPERSEDED {
		t.Errorf("Expected SUPERSEDED status on previous Release version. Got %v", oldStatus)
	}
}

func TestUpdateReleaseNoHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
	}

	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if hl := res.Release.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}

}

func TestUpdateReleaseNoChanges(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart:        rel.GetChart(),
	}

	_, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
}

func TestRollbackReleaseNoHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rel.Hooks = []*release.Hook{
		{
			Name:     "test-cm",
			Kind:     "ConfigMap",
			Path:     "test-cm",
			Manifest: manifestWithRollbackHooks,
			Events: []release.Hook_Event{
				release.Hook_PRE_ROLLBACK,
				release.Hook_POST_ROLLBACK,
			},
		},
	}
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
	}

	res, err := rs.RollbackRelease(c, req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}

	if hl := res.Release.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}
}

func TestRollbackWithReleaseVersion(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Version:      1,
	}

	_, err := rs.RollbackRelease(c, req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}
}

func TestRollbackRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	upgradedRel.Hooks = []*release.Hook{
		{
			Name:     "test-cm",
			Kind:     "ConfigMap",
			Path:     "test-cm",
			Manifest: manifestWithRollbackHooks,
			Events: []release.Hook_Event{
				release.Hook_PRE_ROLLBACK,
				release.Hook_POST_ROLLBACK,
			},
		},
	}

	upgradedRel.Manifest = "hello world"
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.RollbackReleaseRequest{
		Name: rel.Name,
	}
	res, err := rs.RollbackRelease(c, req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}

	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	if res.Release.Name != rel.Name {
		t.Errorf("Updated release name does not match previous release name. Expected %s, got %s", rel.Name, res.Release.Name)
	}

	if res.Release.Namespace != rel.Namespace {
		t.Errorf("Expected release namespace '%s', got '%s'.", rel.Namespace, res.Release.Namespace)
	}

	if res.Release.Version != 3 {
		t.Errorf("Expected release version to be %v, got %v", 3, res.Release.Version)
	}

	updated, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	if len(updated.Hooks) != 2 {
		t.Fatalf("Expected 2 hooks, got %d", len(updated.Hooks))
	}

	if updated.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	anotherUpgradedRelease := upgradeReleaseVersion(upgradedRel)
	rs.env.Releases.Update(upgradedRel)
	rs.env.Releases.Create(anotherUpgradedRelease)

	res, err = rs.RollbackRelease(c, req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}

	updated, err = rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	if len(updated.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(updated.Hooks))
	}

	if updated.Hooks[0].Manifest != manifestWithRollbackHooks {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	if res.Release.Version != 4 {
		t.Errorf("Expected release version to be %v, got %v", 3, res.Release.Version)
	}

	if updated.Hooks[0].Events[0] != release.Hook_PRE_ROLLBACK {
		t.Errorf("Expected event 0 to be pre rollback")
	}

	if updated.Hooks[0].Events[1] != release.Hook_POST_ROLLBACK {
		t.Errorf("Expected event 1 to be post rollback")
	}

	if len(res.Release.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res.Release)
	}

	if len(updated.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(updated.Manifest, "hello world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if res.Release.Info.Description != "Rollback to 2" {
		t.Errorf("Expected rollback to 2, got %q", res.Release.Info.Description)
	}

}

func TestUninstallRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name: "angry-panda",
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
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

	if res.Release.Info.Description != "Deletion complete" {
		t.Errorf("Expected Deletion complete, got %q", res.Release.Info.Description)
	}
}

func TestUninstallPurgeRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.UninstallReleaseRequest{
		Name:  "angry-panda",
		Purge: true,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	if res.Release.Name != "angry-panda" {
		t.Errorf("Expected angry-panda, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Release.Info.Deleted.Seconds <= 0 {
		t.Errorf("Expected valid UNIX date, got %d", res.Release.Info.Deleted.Seconds)
	}
	rels, err := rs.GetHistory(helm.NewContext(), &services.GetHistoryRequest{Name: "angry-panda"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels.Releases) != 0 {
		t.Errorf("Expected no releases in storage, got %d", len(rels.Releases))
	}
}

func TestUninstallPurgeDeleteRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name: "angry-panda",
	}

	_, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	req2 := &services.UninstallReleaseRequest{
		Name:  "angry-panda",
		Purge: true,
	}

	_, err2 := rs.UninstallRelease(c, req2)
	if err2 != nil && err2.Error() != "'angry-panda' has no deployed releases" {
		t.Errorf("Failed uninstall: %s", err2)
	}
}

func TestUninstallReleaseWithKeepPolicy(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	name := "angry-bunny"
	rs.env.Releases.Create(releaseWithKeepStub(name))

	req := &services.UninstallReleaseRequest{
		Name: name,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	if res.Release.Name != name {
		t.Errorf("Expected angry-bunny, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Info == "" {
		t.Errorf("Expected response info to not be empty")
	} else {
		if !strings.Contains(res.Info, "[ConfigMap] test-cm-keep") {
			t.Errorf("unexpected output: %s", res.Info)
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

func TestUninstallReleaseNoHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name:         "angry-panda",
		DisableHooks: true,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed uninstall: %s", err)
	}

	// The default value for a protobuf timestamp is nil.
	if res.Release.Hooks[0].LastRun != nil {
		t.Errorf("Expected LastRun to be zero, got %d.", res.Release.Hooks[0].LastRun.Seconds)
	}
}

func TestGetReleaseContent(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseContent(c, &services.GetReleaseContentRequest{Name: rel.Name, Version: 1})
	if err != nil {
		t.Errorf("Error getting release content: %s", err)
	}

	if res.Release.Chart.Metadata.Name != rel.Chart.Metadata.Name {
		t.Errorf("Expected %q, got %q", rel.Chart.Metadata.Name, res.Release.Chart.Metadata.Name)
	}
}

func TestGetReleaseStatus(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseStatus(c, &services.GetReleaseStatusRequest{Name: rel.Name, Version: 1})
	if err != nil {
		t.Errorf("Error getting release content: %s", err)
	}

	if res.Name != rel.Name {
		t.Errorf("Expected name %q, got %q", rel.Name, res.Name)
	}
	if res.Info.Status.Code != release.Status_DEPLOYED {
		t.Errorf("Expected %d, got %d", release.Status_DEPLOYED, res.Info.Status.Code)
	}
}

func TestGetReleaseStatusDeleted(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rel.Info.Status.Code = release.Status_DELETED
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseStatus(c, &services.GetReleaseStatusRequest{Name: rel.Name, Version: 1})
	if err != nil {
		t.Fatalf("Error getting release content: %s", err)
	}

	if res.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected %d, got %d", release.Status_DELETED, res.Info.Status.Code)
	}
}

func TestListReleases(t *testing.T) {
	rs := rsFixture()
	num := 7
	for i := 0; i < num; i++ {
		rel := releaseStub()
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

func TestListReleasesByStatus(t *testing.T) {
	rs := rsFixture()
	stubs := []*release.Release{
		namedReleaseStub("kamal", release.Status_DEPLOYED),
		namedReleaseStub("astrolabe", release.Status_DELETED),
		namedReleaseStub("octant", release.Status_FAILED),
		namedReleaseStub("sextant", release.Status_UNKNOWN),
	}
	for _, stub := range stubs {
		if err := rs.env.Releases.Create(stub); err != nil {
			t.Fatalf("Could not create stub: %s", err)
		}
	}

	tests := []struct {
		statusCodes []release.Status_Code
		names       []string
	}{
		{
			names:       []string{"kamal"},
			statusCodes: []release.Status_Code{release.Status_DEPLOYED},
		},
		{
			names:       []string{"astrolabe"},
			statusCodes: []release.Status_Code{release.Status_DELETED},
		},
		{
			names:       []string{"kamal", "octant"},
			statusCodes: []release.Status_Code{release.Status_DEPLOYED, release.Status_FAILED},
		},
		{
			names: []string{"kamal", "astrolabe", "octant", "sextant"},
			statusCodes: []release.Status_Code{
				release.Status_DEPLOYED,
				release.Status_DELETED,
				release.Status_FAILED,
				release.Status_UNKNOWN,
			},
		},
	}

	for i, tt := range tests {
		mrs := &mockListServer{}
		if err := rs.ListReleases(&services.ListReleasesRequest{StatusCodes: tt.statusCodes, Offset: "", Limit: 64}, mrs); err != nil {
			t.Fatalf("Failed listing %d: %s", i, err)
		}

		if len(tt.names) != len(mrs.val.Releases) {
			t.Fatalf("Expected %d releases, got %d", len(tt.names), len(mrs.val.Releases))
		}

		for _, name := range tt.names {
			found := false
			for _, rel := range mrs.val.Releases {
				if rel.Name == name {
					found = true
				}
			}
			if !found {
				t.Errorf("%d: Did not find name %q", i, name)
			}
		}
	}
}

func TestListReleasesSort(t *testing.T) {
	rs := rsFixture()

	// Put them in by reverse order so that the mock doesn't "accidentally"
	// sort.
	num := 7
	for i := num; i > 0; i-- {
		rel := releaseStub()
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
		rel := releaseStub()
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

func TestReleasesNamespace(t *testing.T) {
	rs := rsFixture()

	names := []string{
		"axon",
		"dendrite",
		"neuron",
		"ribosome",
	}

	namespaces := []string{
		"default",
		"test123",
		"test123",
		"cerebellum",
	}
	num := 4
	for i := 0; i < num; i++ {
		rel := releaseStub()
		rel.Name = names[i]
		rel.Namespace = namespaces[i]
		if err := rs.env.Releases.Create(rel); err != nil {
			t.Fatalf("Could not store mock release: %s", err)
		}
	}

	mrs := &mockListServer{}
	req := &services.ListReleasesRequest{
		Offset:    "",
		Limit:     64,
		Namespace: "test123",
	}

	if err := rs.ListReleases(req, mrs); err != nil {
		t.Fatalf("Failed listing: %s", err)
	}

	if len(mrs.val.Releases) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(mrs.val.Releases))
	}
}

func TestRunReleaseTest(t *testing.T) {
	rs := rsFixture()
	rel := namedReleaseStub("nemo", release.Status_DEPLOYED)
	rs.env.Releases.Create(rel)

	req := &services.TestReleaseRequest{Name: "nemo", Timeout: 2}
	err := rs.RunReleaseTest(req, mockRunReleaseTestServer{})
	if err != nil {
		t.Fatalf("failed to run release tests on %s: %s", rel.Name, err)
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
