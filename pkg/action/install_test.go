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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/internal/test"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type nameTemplateTestCase struct {
	tpl              string
	expected         string
	expectedErrorStr string
}

func installAction(t *testing.T) *Install {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "test-install-release"

	return instAction
}

func TestInstallRelease(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(), vals)
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

func TestInstallReleaseWithValues(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	userVals := map[string]interface{}{
		"nestedKey": map[string]interface{}{
			"simpleKey": "simpleValue",
		},
	}
	expectedUserValues := map[string]interface{}{
		"nestedKey": map[string]interface{}{
			"simpleKey": "simpleValue",
		},
	}
	res, err := instAction.Run(buildChart(withSampleValues()), userVals)
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
	is.Equal("Install complete", rel.Info.Description)
	is.Equal(expectedUserValues, rel.Config)
}

func TestInstallReleaseClientOnly(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ClientOnly = true
	instAction.Run(buildChart(), nil) // disregard output

	is.Equal(instAction.cfg.Capabilities, chartutil.DefaultCapabilities)
	is.Equal(instAction.cfg.KubeClient, &kubefake.PrintingKubeClient{Out: ioutil.Discard})
}

func TestInstallRelease_NoName(t *testing.T) {
	instAction := installAction(t)
	instAction.ReleaseName = ""
	vals := map[string]interface{}{}
	_, err := instAction.Run(buildChart(), vals)
	if err == nil {
		t.Fatal("expected failure when no name is specified")
	}
	assert.Contains(t, err.Error(), "name is required")
}

func TestInstallRelease_WithNotes(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withNotes("note here")), vals)
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
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withNotes("got-{{.Release.Name}}")), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)

	expectedNotes := fmt.Sprintf("got-%s", res.Name)
	is.Equal(expectedNotes, rel.Info.Notes)
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_WithChartAndDependencyParentNotes(t *testing.T) {
	// Regression: Make sure that the child's notes don't override the parent's
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withNotes("parent"), withDependency(withNotes("child"))), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Equal("with-notes", rel.Name)
	is.NoError(err)
	is.Equal("parent", rel.Info.Notes)
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_WithChartAndDependencyAllNotes(t *testing.T) {
	// Regression: Make sure that the child's notes don't override the parent's
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	instAction.SubNotes = true
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withNotes("parent"), withDependency(withNotes("child"))), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Equal("with-notes", rel.Name)
	is.NoError(err)
	// test run can return as either 'parent\nchild' or 'child\nparent'
	if !strings.Contains(rel.Info.Notes, "parent") && !strings.Contains(rel.Info.Notes, "child") {
		t.Fatalf("Expected 'parent\nchild' or 'child\nparent', got '%s'", rel.Info.Notes)
	}
	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallRelease_DryRun(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DryRun = true
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withSampleTemplates()), vals)
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
	is.True(res.Hooks[0].LastRun.CompletedAt.IsZero(), "expect hook to not be marked as run")
	is.Equal(res.Info.Description, "Dry run complete")
}

func TestInstallRelease_NoHooks(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DisableHooks = true
	instAction.ReleaseName = "no-hooks"
	instAction.cfg.Releases.Create(releaseStub())

	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.True(res.Hooks[0].LastRun.CompletedAt.IsZero(), "hooks should not run with no-hooks")
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WatchUntilReadyError = fmt.Errorf("Failed watch")
	instAction.cfg.KubeClient = failer

	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "failed post-install")
	is.Equal(release.StatusFailed, res.Info.Status)
}

func TestInstallRelease_ReplaceRelease(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.Replace = true

	rel := releaseStub()
	rel.Info.Status = release.StatusUninstalled
	instAction.cfg.Releases.Create(rel)
	instAction.ReleaseName = rel.Name

	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(), vals)
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
	vals := map[string]interface{}{}
	_, err := instAction.Run(buildChart(withKube(">=0.0.0")), vals)
	is.NoError(err)

	// This should fail for a few hundred years
	instAction.ReleaseName = "should-fail"
	vals = map[string]interface{}{}
	_, err = instAction.Run(buildChart(withKube(">=99.0.0")), vals)
	is.Error(err)
	is.Contains(err.Error(), "chart requires kubernetesVersion")
}

func TestInstallRelease_Wait(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "come-fail-away"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	instAction.cfg.KubeClient = failer
	instAction.Wait = true
	vals := map[string]interface{}{}

	res, err := instAction.Run(buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestInstallRelease_Atomic(t *testing.T) {
	is := assert.New(t)

	t.Run("atomic uninstall succeeds", func(t *testing.T) {
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = fmt.Errorf("I timed out")
		instAction.cfg.KubeClient = failer
		instAction.Atomic = true
		vals := map[string]interface{}{}

		res, err := instAction.Run(buildChart(), vals)
		is.Error(err)
		is.Contains(err.Error(), "I timed out")
		is.Contains(err.Error(), "atomic")

		// Now make sure it isn't in storage any more
		_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
		is.Error(err)
		is.Equal(err, driver.ErrReleaseNotFound)
	})

	t.Run("atomic uninstall fails", func(t *testing.T) {
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away-with-me"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = fmt.Errorf("I timed out")
		failer.DeleteError = fmt.Errorf("uninstall fail")
		instAction.cfg.KubeClient = failer
		instAction.Atomic = true
		vals := map[string]interface{}{}

		_, err := instAction.Run(buildChart(), vals)
		is.Error(err)
		is.Contains(err.Error(), "I timed out")
		is.Contains(err.Error(), "uninstall fail")
		is.Contains(err.Error(), "an error occurred while uninstalling the release")
	})
}

func TestNameTemplate(t *testing.T) {
	testCases := []nameTemplateTestCase{
		// Just a straight up nop please
		{
			tpl:              "foobar",
			expected:         "foobar",
			expectedErrorStr: "",
		},
		// Random numbers at the end for fun & profit
		{
			tpl:              "foobar-{{randNumeric 6}}",
			expected:         "foobar-[0-9]{6}$",
			expectedErrorStr: "",
		},
		// Random numbers in the middle for fun & profit
		{
			tpl:              "foobar-{{randNumeric 4}}-baz",
			expected:         "foobar-[0-9]{4}-baz$",
			expectedErrorStr: "",
		},
		// No such function
		{
			tpl:              "foobar-{{randInt}}",
			expected:         "",
			expectedErrorStr: "function \"randInt\" not defined",
		},
		// Invalid template
		{
			tpl:              "foobar-{{",
			expected:         "",
			expectedErrorStr: "unexpected unclosed action",
		},
	}

	for _, tc := range testCases {

		n, err := TemplateName(tc.tpl)
		if err != nil {
			if tc.expectedErrorStr == "" {
				t.Errorf("Was not expecting error, but got: %v", err)
				continue
			}
			re, compErr := regexp.Compile(tc.expectedErrorStr)
			if compErr != nil {
				t.Errorf("Expected error string failed to compile: %v", compErr)
				continue
			}
			if !re.MatchString(err.Error()) {
				t.Errorf("Error didn't match for %s expected %s but got %v", tc.tpl, tc.expectedErrorStr, err)
				continue
			}
		}
		if err == nil && tc.expectedErrorStr != "" {
			t.Errorf("Was expecting error %s but didn't get an error back", tc.expectedErrorStr)
		}

		if tc.expected != "" {
			re, err := regexp.Compile(tc.expected)
			if err != nil {
				t.Errorf("Expected string failed to compile: %v", err)
				continue
			}
			if !re.MatchString(n) {
				t.Errorf("Returned name didn't match for %s expected %s but got %s", tc.tpl, tc.expected, n)
			}
		}
	}
}

func TestInstallReleaseOutputDir(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	vals := map[string]interface{}{}

	dir, err := ioutil.TempDir("", "output-dir")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	instAction.OutputDir = dir

	_, err = instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	_, err = os.Stat(filepath.Join(dir, "hello/templates/goodbye"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello/templates/hello"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello/templates/with-partials"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello/templates/rbac"))
	is.NoError(err)

	test.AssertGoldenFile(t, filepath.Join(dir, "hello/templates/rbac"), "rbac.txt")

	_, err = os.Stat(filepath.Join(dir, "hello/templates/empty"))
	is.True(os.IsNotExist(err))
}
