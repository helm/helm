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
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v3/internal/test"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmtime "helm.sh/helm/v3/pkg/time"
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
	req := require.New(t)

	instAction := installAction(t)
	vals := map[string]interface{}{}
	ctx, done := context.WithCancel(context.Background())
	res, err := instAction.RunWithContext(ctx, buildChart(), vals)
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

	// Detecting previous bug where context termination after successful release
	// caused release to fail.
	done()
	time.Sleep(time.Millisecond * 100)
	lastRelease, err := instAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	is.Equal(lastRelease.Info.Status, release.StatusDeployed)
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
	assert.Contains(t, err.Error(), "no name provided")
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

// Regression test for #7955: Lookup must not connect to Kubernetes on a dry-run.
func TestInstallRelease_DryRun_Lookup(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DryRun = true
	vals := map[string]interface{}{}

	mockChart := buildChart(withSampleTemplates())
	mockChart.Templates = append(mockChart.Templates, &chart.File{
		Name: "templates/lookup",
		Data: []byte(`goodbye: {{ lookup "v1" "Namespace" "" "___" }}`),
	})

	res, err := instAction.Run(mockChart, vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.Contains(res.Manifest, "goodbye: map[]")
}

func TestInstallReleaseIncorrectTemplate_DryRun(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.DryRun = true
	vals := map[string]interface{}{}
	_, err := instAction.Run(buildChart(withSampleIncludingIncorrectTemplates()), vals)
	expectedErr := "\"hello/templates/incorrect\" at <.Values.bad.doh>: nil pointer evaluating interface {}.doh"
	if err == nil {
		t.Fatalf("Install should fail containing error: %s", expectedErr)
	}
	if err != nil {
		is.Contains(err.Error(), expectedErr)
	}
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
	is.Contains(err.Error(), "chart requires kubeVersion")
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
func TestInstallRelease_Wait_Interrupted(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.Wait = true
	vals := map[string]interface{}{}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	time.AfterFunc(time.Second, cancel)

	res, err := instAction.RunWithContext(ctx, buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "Release \"interrupted-release\" failed: context canceled")
	is.Equal(res.Info.Status, release.StatusFailed)
}
func TestInstallRelease_WaitForJobs(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "come-fail-away"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	instAction.cfg.KubeClient = failer
	instAction.Wait = true
	instAction.WaitForJobs = true
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
func TestInstallRelease_Atomic_Interrupted(t *testing.T) {

	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.Atomic = true
	vals := map[string]interface{}{}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	time.AfterFunc(time.Second, cancel)

	res, err := instAction.RunWithContext(ctx, buildChart(), vals)
	is.Error(err)
	is.Contains(err.Error(), "context canceled")
	is.Contains(err.Error(), "atomic")
	is.Contains(err.Error(), "uninstalled")

	// Now make sure it isn't in storage any more
	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Error(err)
	is.Equal(err, driver.ErrReleaseNotFound)

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
			tpl:              "foobar-{{randInteger}}",
			expected:         "",
			expectedErrorStr: "function \"randInteger\" not defined",
		},
		// Invalid template
		{
			tpl:              "foobar-{{",
			expected:         "",
			expectedErrorStr: "template: name-template:1: unclosed action",
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

func TestInstallOutputDirWithReleaseName(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	vals := map[string]interface{}{}

	dir, err := ioutil.TempDir("", "output-dir")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	instAction.OutputDir = dir
	instAction.UseReleaseName = true
	instAction.ReleaseName = "madra"

	newDir := filepath.Join(dir, instAction.ReleaseName)

	_, err = instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	_, err = os.Stat(filepath.Join(newDir, "hello/templates/goodbye"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello/templates/hello"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello/templates/with-partials"))
	is.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello/templates/rbac"))
	is.NoError(err)

	test.AssertGoldenFile(t, filepath.Join(newDir, "hello/templates/rbac"), "rbac.txt")

	_, err = os.Stat(filepath.Join(newDir, "hello/templates/empty"))
	is.True(os.IsNotExist(err))
}

func TestNameAndChart(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	chartName := "./foo"

	name, chrt, err := instAction.NameAndChart([]string{chartName})
	if err != nil {
		t.Fatal(err)
	}
	is.Equal(instAction.ReleaseName, name)
	is.Equal(chartName, chrt)

	instAction.GenerateName = true
	_, _, err = instAction.NameAndChart([]string{"foo", chartName})
	if err == nil {
		t.Fatal("expected an error")
	}
	is.Equal("cannot set --generate-name and also specify a name", err.Error())

	instAction.GenerateName = false
	instAction.NameTemplate = "{{ . }}"
	_, _, err = instAction.NameAndChart([]string{"foo", chartName})
	if err == nil {
		t.Fatal("expected an error")
	}
	is.Equal("cannot set --name-template and also specify a name", err.Error())

	instAction.NameTemplate = ""
	instAction.ReleaseName = ""
	_, _, err = instAction.NameAndChart([]string{chartName})
	if err == nil {
		t.Fatal("expected an error")
	}
	is.Equal("must either provide a name or specify --generate-name", err.Error())

	instAction.NameTemplate = ""
	instAction.ReleaseName = ""
	_, _, err = instAction.NameAndChart([]string{"foo", chartName, "bar"})
	if err == nil {
		t.Fatal("expected an error")
	}
	is.Equal("expected at most two arguments, unexpected arguments: bar", err.Error())
}

func TestNameAndChartGenerateName(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)

	instAction.ReleaseName = ""
	instAction.GenerateName = true

	tests := []struct {
		Name         string
		Chart        string
		ExpectedName string
	}{
		{
			"local filepath",
			"./chart",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
		{
			"dot filepath",
			".",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
		{
			"empty filepath",
			"",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
		{
			"packaged chart",
			"chart.tgz",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
		{
			"packaged chart with .tar.gz extension",
			"chart.tar.gz",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
		{
			"packaged chart with local extension",
			"./chart.tgz",
			fmt.Sprintf("chart-%d", helmtime.Now().Unix()),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			name, chrt, err := instAction.NameAndChart([]string{tc.Chart})
			if err != nil {
				t.Fatal(err)
			}

			is.Equal(tc.ExpectedName, name)
			is.Equal(tc.Chart, chrt)
		})
	}
}
