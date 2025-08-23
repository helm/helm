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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"

	"helm.sh/helm/v4/internal/test"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	helmtime "helm.sh/helm/v4/pkg/time"
)

type nameTemplateTestCase struct {
	tpl              string
	expected         string
	expectedErrorStr string
}

var goroutinesLock sync.Mutex

func createDummyResourceList(owned bool) kube.ResourceList {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummyName",
			Namespace: "spaced",
		},
	}

	if owned {
		obj.Labels = map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
		}
		obj.Annotations = map[string]string{
			"meta.helm.sh/release-name":      "test-install-release",
			"meta.helm.sh/release-namespace": "spaced",
		}
	}

	resInfo := resource.Info{
		Name:      "dummyName",
		Namespace: "spaced",
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
			GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Scope:            meta.RESTScopeNamespace,
		},
		Object: obj,
	}
	body := io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(appsv1Codec, obj))))

	resInfo.Client = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Group: "apps", Version: "v1"},
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			header := http.Header{}
			header.Set("Content-Type", kuberuntime.ContentTypeJSON)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       body,
			}, nil
		}),
	}
	var resourceList kube.ResourceList
	resourceList.Append(&resInfo)
	return resourceList
}

func installActionWithConfig(config *Configuration) *Install {
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "test-install-release"

	return instAction
}

func installAction(t *testing.T) *Install {
	t.Helper()
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
	ctx, done := context.WithCancel(t.Context())
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

func TestInstallReleaseWithTakeOwnership_ResourceNotOwned(t *testing.T) {
	// This test will test checking ownership of a resource
	// returned by the fake client. If the resource is not
	// owned by the chart, ownership is taken.
	// To verify ownership has been taken, the fake client
	// needs to store state which is a bigger rewrite.
	// TODO: Ensure fake kube client stores state. Maybe using
	// "k8s.io/client-go/kubernetes/fake" could be sufficient? i.e
	// "Client{Namespace: namespace, kubeClient: k8sfake.NewClientset()}"

	is := assert.New(t)

	// Resource list from cluster is NOT owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(false))
	instAction := installActionWithConfig(config)
	instAction.TakeOwnership = true
	res, err := instAction.Run(buildChart(), nil)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)

	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallReleaseWithTakeOwnership_ResourceOwned(t *testing.T) {
	is := assert.New(t)

	// Resource list from cluster is owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(true))
	instAction := installActionWithConfig(config)
	instAction.TakeOwnership = false
	res, err := instAction.Run(buildChart(), nil)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	rel, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	is.NoError(err)

	is.Equal(rel.Info.Description, "Install complete")
}

func TestInstallReleaseWithTakeOwnership_ResourceOwnedNoFlag(t *testing.T) {
	is := assert.New(t)

	// Resource list from cluster is NOT owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(false))
	instAction := installActionWithConfig(config)
	_, err := instAction.Run(buildChart(), nil)
	is.Error(err)
	is.Contains(err.Error(), "unable to continue with install")
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
	is.Equal(instAction.cfg.KubeClient, &kubefake.PrintingKubeClient{Out: io.Discard})
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

func TestInstallRelease_DryRunHiddenSecret(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)

	// First perform a normal dry-run with the secret and confirm its presence.
	instAction.DryRun = true
	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}
	is.Contains(res.Manifest, "---\n# Source: hello/templates/secret.yaml\napiVersion: v1\nkind: Secret")

	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Error(err)
	is.Equal(res.Info.Description, "Dry run complete")

	// Perform a dry-run where the secret should not be present
	instAction.HideSecret = true
	vals = map[string]interface{}{}
	res2, err := instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.NotContains(res2.Manifest, "---\n# Source: hello/templates/secret.yaml\napiVersion: v1\nkind: Secret")

	_, err = instAction.cfg.Releases.Get(res2.Name, res2.Version)
	is.Error(err)
	is.Equal(res2.Info.Description, "Dry run complete")

	// Ensure there is an error when HideSecret True but not in a dry-run mode
	instAction.DryRun = false
	vals = map[string]interface{}{}
	_, err = instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	if err == nil {
		t.Fatalf("Did not get expected an error when dry-run false and hide secret is true")
	}
}

// Regression test for #7955
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
	expectedErr := `hello/templates/incorrect:1:10
  executing "hello/templates/incorrect" at <.Values.bad.doh>:
    nil pointer evaluating interface {}.doh`
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
	outBuffer := &bytes.Buffer{}
	failer.PrintingKubeClient = kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: outBuffer}

	vals := map[string]interface{}{}
	res, err := instAction.Run(buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "failed post-install")
	is.Equal("", outBuffer.String())
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
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]interface{}{}

	goroutinesLock.Lock()
	defer goroutinesLock.Unlock()

	goroutines := runtime.NumGoroutine()

	res, err := instAction.Run(buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)

	is.Equal(goroutines, runtime.NumGoroutine())
}
func TestInstallRelease_Wait_Interrupted(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]interface{}{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	goroutinesLock.Lock()
	defer goroutinesLock.Unlock()

	goroutines := runtime.NumGoroutine()

	_, err := instAction.RunWithContext(ctx, buildChart(), vals)
	is.Error(err)
	is.Contains(err.Error(), "context canceled")

	is.Equal(goroutines+1, runtime.NumGoroutine()) // installation goroutine still is in background
	time.Sleep(10 * time.Second)                   // wait for goroutine to finish
	is.Equal(goroutines, runtime.NumGoroutine())
}
func TestInstallRelease_WaitForJobs(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "come-fail-away"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	instAction.cfg.KubeClient = failer
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	instAction.WaitForJobs = true
	vals := map[string]interface{}{}

	res, err := instAction.Run(buildChart(), vals)
	is.Error(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestInstallRelease_RollbackOnFailure(t *testing.T) {
	is := assert.New(t)

	t.Run("rollback-on-failure uninstall succeeds", func(t *testing.T) {
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = fmt.Errorf("I timed out")
		instAction.cfg.KubeClient = failer
		instAction.RollbackOnFailure = true
		// disabling hooks to avoid an early fail when
		// WaitForDelete is called on the pre-delete hook execution
		instAction.DisableHooks = true
		vals := map[string]interface{}{}

		res, err := instAction.Run(buildChart(), vals)
		is.Error(err)
		is.Contains(err.Error(), "I timed out")
		is.Contains(err.Error(), "rollback-on-failure")

		// Now make sure it isn't in storage anymore
		_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
		is.Error(err)
		is.Equal(err, driver.ErrReleaseNotFound)
	})

	t.Run("rollback-on-failure uninstall fails", func(t *testing.T) {
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away-with-me"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = fmt.Errorf("I timed out")
		failer.DeleteError = fmt.Errorf("uninstall fail")
		instAction.cfg.KubeClient = failer
		instAction.RollbackOnFailure = true
		vals := map[string]interface{}{}

		_, err := instAction.Run(buildChart(), vals)
		is.Error(err)
		is.Contains(err.Error(), "I timed out")
		is.Contains(err.Error(), "uninstall fail")
		is.Contains(err.Error(), "an error occurred while uninstalling the release")
	})
}
func TestInstallRelease_RollbackOnFailure_Interrupted(t *testing.T) {

	is := assert.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.RollbackOnFailure = true
	vals := map[string]interface{}{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	goroutinesLock.Lock()
	defer goroutinesLock.Unlock()

	goroutines := runtime.NumGoroutine()

	res, err := instAction.RunWithContext(ctx, buildChart(), vals)
	is.Error(err)
	is.Contains(err.Error(), "context canceled")
	is.Contains(err.Error(), "rollback-on-failure")
	is.Contains(err.Error(), "uninstalled")

	// Now make sure it isn't in storage anymore
	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	is.Error(err)
	is.Equal(err, driver.ErrReleaseNotFound)
	is.Equal(goroutines+1, runtime.NumGoroutine()) // installation goroutine still is in background
	time.Sleep(10 * time.Second)                   // wait for goroutine to finish
	is.Equal(goroutines, runtime.NumGoroutine())

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

	dir := t.TempDir()

	instAction.OutputDir = dir

	_, err := instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
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
	is.True(errors.Is(err, fs.ErrNotExist))
}

func TestInstallOutputDirWithReleaseName(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	vals := map[string]interface{}{}

	dir := t.TempDir()

	instAction.OutputDir = dir
	instAction.UseReleaseName = true
	instAction.ReleaseName = "madra"

	newDir := filepath.Join(dir, instAction.ReleaseName)

	_, err := instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
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
	is.True(errors.Is(err, fs.ErrNotExist))
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

func TestInstallWithLabels(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.Labels = map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	res, err := instAction.Run(buildChart(), nil)
	if err != nil {
		t.Fatalf("Failed install: %s", err)
	}

	is.Equal(instAction.Labels, res.Labels)
}

func TestInstallWithSystemLabels(t *testing.T) {
	is := assert.New(t)
	instAction := installAction(t)
	instAction.Labels = map[string]string{
		"owner": "val1",
		"key2":  "val2",
	}
	_, err := instAction.Run(buildChart(), nil)
	if err == nil {
		t.Fatal("expected an error")
	}

	is.Equal(fmt.Errorf("user supplied labels contains system reserved label name. System labels: %+v", driver.GetSystemLabels()), err)
}

func TestUrlEqual(t *testing.T) {
	is := assert.New(t)

	tests := []struct {
		name     string
		url1     string
		url2     string
		expected bool
	}{
		{
			name:     "identical URLs",
			url1:     "https://example.com:443",
			url2:     "https://example.com:443",
			expected: true,
		},
		{
			name:     "same host, scheme, default HTTPS port vs explicit",
			url1:     "https://example.com",
			url2:     "https://example.com:443",
			expected: true,
		},
		{
			name:     "same host, scheme, default HTTP port vs explicit",
			url1:     "http://example.com",
			url2:     "http://example.com:80",
			expected: true,
		},
		{
			name:     "different schemes",
			url1:     "http://example.com",
			url2:     "https://example.com",
			expected: false,
		},
		{
			name:     "different hosts",
			url1:     "https://example.com",
			url2:     "https://www.example.com",
			expected: false,
		},
		{
			name:     "different ports",
			url1:     "https://example.com:8080",
			url2:     "https://example.com:9090",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u1, err := url.Parse(tc.url1)
			if err != nil {
				t.Fatalf("Failed to parse URL1 %s: %v", tc.url1, err)
			}
			u2, err := url.Parse(tc.url2)
			if err != nil {
				t.Fatalf("Failed to parse URL2 %s: %v", tc.url2, err)
			}

			is.Equal(tc.expected, urlEqual(u1, u2))
		})
	}
}
