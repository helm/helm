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

package action

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/helm/pkg/hapi/release"
)

func installAction(t *testing.T) *Install {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "test-install-release"

	return instAction
}

var mockEmptyVals = func() map[string]interface{} { return map[string]interface{}{} }

func TestInstallRelease(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	res, err := instAction.Run(buildChart(), mockEmptyVals())
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	is.Equal(res.Name, "test-install-release", "Expected release name.")
	is.Equal(res.Namespace, "spaced")

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)

	is.Len(rel.Hooks, 1)
	is.Equal(rel.Hooks[0].Manifest, manifestWithHook)
	is.Equal(rel.Hooks[0].Events[0], release.HookPostInstall)
	is.Equal(rel.Hooks[0].Events[1], release.HookPreDelete, "Expected event 0 is pre-delete")

	is.NotEqual(len(res.Manifest), 0)
	is.NotEqual(len(rel.Manifest), 0)
	is.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_NoName(t *testing.T) {
	instAction := installAction(t)
	instAction.ReleaseName = ""
	_, err := instAction.Run(buildChart(), mockEmptyVals())
	if err == nil {
		t.Fatal("expected failure when no name is specified")
	}
	assert.Contains(t, err.Error(), "name is required")
}

func TestInstallRelease_WithNotes(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	res, err := instAction.Run(buildChart(withNotes("note here")), mockEmptyVals())
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.Equal(res.Name, "with-notes")
	is.Equal(res.Namespace, "spaced")

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)
	is.Len(rel.Hooks, 1)
	is.Equal(rel.Hooks[0].Manifest, manifestWithHook)
	is.Equal(rel.Hooks[0].Events[0], release.HookPostInstall)
	is.Equal(rel.Hooks[0].Events[1], release.HookPreDelete, "Expected event 0 is pre-delete")
	is.NotEqual(len(res.Manifest), 0)
	is.NotEqual(len(rel.Manifest), 0)
	is.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Equal(rel.Info.Description, "Install complete")

	is.Equal(rel.Info.Notes, "note here")
}

func TestInstallRelease_WithNotesRendered(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	res, err := instAction.Run(buildChart(withNotes("got-{{.Release.Name}}")), mockEmptyVals())
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)

	expectedNotes := fmt.Sprintf("got-%s", res.Name)
	is.Equal(expectedNotes, rel.Info.Notes)
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_WithChartAndDependencyNotes(t *testing.T) {
	// Regression: Make sure that the child's notes don't override the parent's
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	res, err := instAction.Run(buildChart(
		withNotes("parent"),
		withDependency(withNotes("child"))),
		mockEmptyVals(),
	)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Equal("with-notes", rel.Name)
	is.NoError(err)
	is.Equal("parent", rel.Info.Notes)
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_DryRun(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DryRun = true
	res, err := instAction.Run(buildChart(withSampleTemplates()), mockEmptyVals())
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.Contains(res.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Contains(res.Manifest, "---\n# Source: hello/templates/goodbye\ngoodbye: world")
	is.Contains(res.Manifest, "hello: Earth")
	is.NotContains(res.Manifest, "hello: {{ template \"_planet\" . }}")
	is.NotContains(res.Manifest, "empty")

	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Error(err)
	is.Len(res.Hooks, 1)
	is.True(res.Hooks[0].LastRun.IsZero(), "expect hook to not be marked as run")
	is.Equal(res.Info.Description, "Dry run complete")
}

func TestInstallRelease_NoHooks(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DisableHooks = true
	instAction.ReleaseName = "no-hooks"
	instAction.cfg.Releases.Create(releaseStub())

	res, err := instAction.Run(buildChart(), mockEmptyVals())
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.True(res.Hooks[0].LastRun.IsZero(), "hooks should not run with no-hooks")
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	instAction.cfg.KubeClient = newHookFailingKubeClient()

	res, err := instAction.Run(buildChart(), mockEmptyVals())
	is.Error(err)
	is.Contains(res.Info.Description, "failed post-install")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestInstallRelease_ReplaceRelease(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.Replace = true

	rel := releaseStub()
	rel.Info.Status = release.StatusUninstalled
	instAction.cfg.Releases.Create(rel)
	instAction.ReleaseName = rel.Name

	res, err := instAction.Run(buildChart(), mockEmptyVals())
	is.NoError(err)

	// This should have been auto-incremented
	is.Equal(2, res.Version)
	is.Equal(res.Name, rel.Name)

	getres, err := instAction.cfg.Releases.Get(rel.Name, res.Version)
	is.NoError(err)
	is.Equal(getres.Info.Status, release.StatusDeployed)
}

func TestInstallRelease_KubeVersion(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	_, err := instAction.Run(buildChart(withKube(">=0.0.0")), mockEmptyVals())
	is.NoError(err)

	// This should fail for a few hundred years
	instAction.ReleaseName = "should-fail"
	_, err = instAction.Run(buildChart(withKube(">=99.0.0")), mockEmptyVals())
	is.Error(err)
	is.Contains(err.Error(), "chart requires kubernetesVersion")
}
