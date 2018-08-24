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

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

func TestInstallRelease(t *testing.T) {
	rs := rsFixture(t)

	req := installRequest()
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Namespace)
	}

	rel, err := rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	if rel.Hooks[0].Events[0] != release.HookPostInstall {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.HookPreDelete {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res)
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
	rs := rsFixture(t)

	req := installRequest(
		withChart(withNotes(notesText)),
	)
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Namespace)
	}

	rel, err := rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	if rel.Info.Notes != notesText {
		t.Fatalf("Expected '%s', got '%s'", notesText, rel.Info.Notes)
	}

	if rel.Hooks[0].Events[0] != release.HookPostInstall {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.HookPreDelete {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res)
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
	rs := rsFixture(t)

	req := installRequest(
		withChart(withNotes(notesText + " {{.Release.Name}}")),
	)
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Name == "" {
		t.Errorf("Expected release name.")
	}
	if res.Namespace != "spaced" {
		t.Errorf("Expected release namespace 'spaced', got '%s'.", res.Namespace)
	}

	rel, err := rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	t.Logf("rel: %v", rel)

	if len(rel.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(rel.Hooks))
	}
	if rel.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", rel.Hooks[0].Manifest)
	}

	expectedNotes := fmt.Sprintf("%s %s", notesText, res.Name)
	if rel.Info.Notes != expectedNotes {
		t.Fatalf("Expected '%s', got '%s'", expectedNotes, rel.Info.Notes)
	}

	if rel.Hooks[0].Events[0] != release.HookPostInstall {
		t.Errorf("Expected event 0 is post install")
	}
	if rel.Hooks[0].Events[1] != release.HookPreDelete {
		t.Errorf("Expected event 0 is pre-delete")
	}

	if len(res.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res)
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

func TestInstallRelease_WithChartAndDependencyNotes(t *testing.T) {
	rs := rsFixture(t)

	req := installRequest(withChart(
		withNotes(notesText),
		withDependency(withNotes(notesText+" child")),
	))
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	if res.Name == "" {
		t.Errorf("Expected release name.")
	}

	rel, err := rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	t.Logf("rel: %v", rel)

	if rel.Info.Notes != notesText {
		t.Fatalf("Expected '%s', got '%s'", notesText, rel.Info.Notes)
	}

	if rel.Info.Description != "Install complete" {
		t.Errorf("unexpected description: %s", rel.Info.Description)
	}
}

func TestInstallRelease_DryRun(t *testing.T) {
	rs := rsFixture(t)

	req := installRequest(withDryRun(),
		withChart(withSampleTemplates()),
	)
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}
	if res.Name == "" {
		t.Errorf("Expected release name.")
	}

	if !strings.Contains(res.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", res.Manifest)
	}

	if !strings.Contains(res.Manifest, "---\n# Source: hello/templates/goodbye\ngoodbye: world") {
		t.Errorf("unexpected output: %s", res.Manifest)
	}

	if !strings.Contains(res.Manifest, "hello: Earth") {
		t.Errorf("Should contain partial content. %s", res.Manifest)
	}

	if strings.Contains(res.Manifest, "hello: {{ template \"_planet\" . }}") {
		t.Errorf("Should not contain partial templates itself. %s", res.Manifest)
	}

	if strings.Contains(res.Manifest, "empty") {
		t.Errorf("Should not contain template data for an empty file. %s", res.Manifest)
	}

	if _, err := rs.Releases.Get(res.Name, res.Version); err == nil {
		t.Errorf("Expected no stored release.")
	}

	if l := len(res.Hooks); l != 1 {
		t.Fatalf("Expected 1 hook, got %d", l)
	}

	if !res.Hooks[0].LastRun.IsZero() {
		t.Error("Expected hook to not be marked as run.")
	}

	if res.Info.Description != "Dry run complete" {
		t.Errorf("unexpected description: %s", res.Info.Description)
	}
}

func TestInstallRelease_NoHooks(t *testing.T) {
	rs := rsFixture(t)
	rs.Releases.Create(releaseStub())

	req := installRequest(withDisabledHooks())
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}

	if !res.Hooks[0].LastRun.IsZero() {
		t.Errorf("Expected that no hooks were run. Got %s", res.Hooks[0].LastRun)
	}
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	rs := rsFixture(t)
	rs.Releases.Create(releaseStub())
	rs.KubeClient = newHookFailingKubeClient()

	req := installRequest()
	res, err := rs.InstallRelease(req)
	if err == nil {
		t.Error("Expected failed install")
	}

	if hl := res.Info.Status; hl != release.StatusFailed {
		t.Errorf("Expected FAILED release. Got %s", hl)
	}
}

func TestInstallRelease_ReuseName(t *testing.T) {
	rs := rsFixture(t)
	rs.Log = t.Logf
	rel := releaseStub()
	rel.Info.Status = release.StatusUninstalled
	rs.Releases.Create(rel)

	req := installRequest(
		withReuseName(),
		withName(rel.Name),
	)
	res, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	if res.Name != rel.Name {
		t.Errorf("expected %q, got %q", rel.Name, res.Name)
	}

	getreq := &hapi.GetReleaseStatusRequest{Name: rel.Name, Version: 0}
	getres, err := rs.GetReleaseStatus(getreq)
	if err != nil {
		t.Errorf("Failed to retrieve release: %s", err)
	}
	if getres.Info.Status != release.StatusDeployed {
		t.Errorf("Release status is %q", getres.Info.Status)
	}
}

func TestInstallRelease_KubeVersion(t *testing.T) {
	rs := rsFixture(t)

	req := installRequest(
		withChart(withKube(">=0.0.0")),
	)
	_, err := rs.InstallRelease(req)
	if err != nil {
		t.Fatalf("Expected valid range. Got %q", err)
	}
}

func TestInstallRelease_WrongKubeVersion(t *testing.T) {
	rs := rsFixture(t)

	req := installRequest(
		withChart(withKube(">=5.0.0")),
	)

	_, err := rs.InstallRelease(req)
	if err == nil {
		t.Fatalf("Expected to fail because of wrong version")
	}

	expect := "chart requires kubernetesVersion"
	if !strings.Contains(err.Error(), expect) {
		t.Errorf("Expected %q to contain %q", err.Error(), expect)
	}
}
