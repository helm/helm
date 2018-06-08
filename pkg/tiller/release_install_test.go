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

package tiller

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/version"
)

func TestHasCRDHook(t *testing.T) {
	tests := []struct {
		hooks  []*release.Hook
		expect bool
	}{
		{
			hooks: []*release.Hook{
				{Events: []release.Hook_Event{release.Hook_PRE_DELETE}},
			},
			expect: false,
		},
		{
			hooks: []*release.Hook{
				{Events: []release.Hook_Event{release.Hook_CRD_INSTALL}},
			},
			expect: true,
		},
		{
			hooks: []*release.Hook{
				{Events: []release.Hook_Event{release.Hook_PRE_UPGRADE, release.Hook_CRD_INSTALL}},
			},
			expect: true,
		},
	}

	for i, tt := range tests {
		if tt.expect != hasCRDHook(tt.hooks) {
			t.Errorf("test %d: expected %t, got %t", i, tt.expect, !tt.expect)
		}
	}
}

func TestInstallRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	req := installRequest()
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

	req := installRequest(
		withChart(withNotes(notesText)),
	)
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

	req := installRequest(
		withChart(withNotes(notesText + " {{.Release.Name}}")),
	)
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

	req := installRequest(
		withChart(withTiller(">=2.2.0")),
	)
	_, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Expected valid range. Got %q", err)
	}
}

func TestInstallRelease_WrongTillerVersion(t *testing.T) {
	version.Version = "2.2.0"
	c := helm.NewContext()
	rs := rsFixture()

	req := installRequest(
		withChart(withTiller("<2.0.0")),
	)
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

	req := installRequest(withChart(
		withNotes(notesText),
		withDependency(withNotes(notesText+" child")),
	))
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

	req := installRequest(withDryRun(),
		withChart(withSampleTemplates()),
	)
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

	req := installRequest(withDisabledHooks())
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	if hl := res.Release.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}
}

func TestInstallRelease_CRDInstallHook(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := installRequest()
	req.Chart.Templates = append(req.Chart.Templates, &chart.Template{
		Name: "templates/crdhook",
		Data: []byte(manifestWithCRDHook),
	})

	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	// The new hook should have been pulled from the manifest.
	if l := len(res.Release.Hooks); l != 2 {
		t.Fatalf("expected 2 hooks, got %d", l)
	}

	expect := "Install complete"
	if got := res.Release.Info.Description; got != expect {
		t.Errorf("Expected Description to be %q, got %q", expect, got)
	}
}

func TestInstallRelease_DryRunCRDInstallHook(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := installRequest(withDryRun())
	req.Chart.Templates = append(req.Chart.Templates, &chart.Template{
		Name: "templates/crdhook",
		Data: []byte(manifestWithCRDHook),
	})

	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	expect := "Validation skipped because CRDs are not installed"
	if res.Release.Info.Description != expect {
		t.Errorf("Expected Description %q, got %q", expect, res.Release.Info.Description)
	}
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())
	rs.env.KubeClient = newHookFailingKubeClient()

	req := installRequest()
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

	req := installRequest(
		withReuseName(),
		withName(rel.Name),
	)
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

func TestInstallRelease_KubeVersion(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	req := installRequest(
		withChart(withKube(">=0.0.0")),
	)
	_, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Fatalf("Expected valid range. Got %q", err)
	}
}

func TestInstallRelease_WrongKubeVersion(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()

	req := installRequest(
		withChart(withKube(">=5.0.0")),
	)

	_, err := rs.InstallRelease(c, req)
	if err == nil {
		t.Fatalf("Expected to fail because of wrong version")
	}

	expect := "Chart requires kubernetesVersion"
	if !strings.Contains(err.Error(), expect) {
		t.Errorf("Expected %q to contain %q", err.Error(), expect)
	}
}

func TestInstallRelease_Description(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	customDescription := "foo"
	req := &services.InstallReleaseRequest{
		Chart:       chartStub(),
		Description: customDescription,
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	if desc := res.Release.Info.Description; desc != customDescription {
		t.Errorf("Expected description %q. Got %q", customDescription, desc)
	}
}
