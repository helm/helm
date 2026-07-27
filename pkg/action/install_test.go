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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"

	ci "helm.sh/helm/v4/pkg/chart"

	"helm.sh/helm/v4/internal/test"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/registry"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
)

type nameTemplateTestCase struct {
	tpl              string
	expected         string
	expectedErrorStr string
}

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

func createDummyCRDList(owned bool) kube.ResourceList {
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
			Resource:         schema.GroupVersionResource{Group: "test", Version: "v1", Resource: "crd"},
			GroupVersionKind: schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "crd"},
			Scope:            meta.RESTScopeNamespace,
		},
		Object: obj,
	}
	body := io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(appsv1Codec, obj))))

	resInfo.Client = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Group: "test", Version: "v1"},
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
	vals := map[string]any{}
	ctx, done := context.WithCancel(t.Context())
	resi, err := instAction.RunWithContext(ctx, buildChart(), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Equal("test-install-release", res.Name, "Expected release name.")
	is.Equal("spaced", res.Namespace)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)

	rel, err := releaserToV1Release(r)
	req.NoError(err)

	is.Len(rel.Hooks, 1)
	is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
	is.Equal(release.HookPostInstall, rel.Hooks[0].Events[0])
	is.Equal(release.HookPreDelete, rel.Hooks[0].Events[1], "Expected event 0 is pre-delete")

	is.NotEmpty(res.Manifest)
	is.NotEmpty(rel.Manifest)
	is.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Equal("Install complete", rel.Info.Description)

	// Detecting previous bug where context termination after successful release
	// caused release to fail.
	done()
	time.Sleep(time.Millisecond * 100)
	lastRelease, err := instAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	lrel, err := releaserToV1Release(lastRelease)
	req.NoError(err)
	is.Equal(rcommon.StatusDeployed, lrel.Info.Status)
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
	req := require.New(t)

	// Resource list from cluster is NOT owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(false))
	instAction := installActionWithConfig(config)
	instAction.TakeOwnership = true
	resi, err := instAction.Run(buildChart(), nil)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)

	rel, err := releaserToV1Release(r)
	req.NoError(err)

	is.Equal("Install complete", rel.Info.Description)
}

func TestInstallReleaseWithTakeOwnership_ResourceOwned(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	// Resource list from cluster is owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(true))
	instAction := installActionWithConfig(config)
	instAction.TakeOwnership = false
	resi, err := instAction.Run(buildChart(), nil)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)

	rel, err := releaserToV1Release(r)
	req.NoError(err)

	is.Equal("Install complete", rel.Info.Description)
}

func TestInstallReleaseWithTakeOwnership_ResourceOwnedNoFlag(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	// Resource list from cluster is NOT owned by helm chart
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(false))
	instAction := installActionWithConfig(config)
	_, err := instAction.Run(buildChart(), nil)
	req.Error(err)
	is.ErrorContains(err, "unable to continue with install")
}

func TestInstallReleaseWithValues(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	userVals := map[string]any{
		"nestedKey": map[string]any{
			"simpleKey": "simpleValue",
		},
	}
	expectedUserValues := map[string]any{
		"nestedKey": map[string]any{
			"simpleKey": "simpleValue",
		},
	}
	resi, err := instAction.Run(buildChart(withSampleValues()), userVals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Equal("test-install-release", res.Name, "Expected release name.")
	is.Equal("spaced", res.Namespace)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)

	rel, err := releaserToV1Release(r)
	req.NoError(err)

	is.Len(rel.Hooks, 1)
	is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
	is.Equal(release.HookPostInstall, rel.Hooks[0].Events[0])
	is.Equal(release.HookPreDelete, rel.Hooks[0].Events[1], "Expected event 0 is pre-delete")

	is.NotEmpty(res.Manifest)
	is.NotEmpty(rel.Manifest)
	is.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Equal("Install complete", rel.Info.Description)
	is.Equal(expectedUserValues, rel.Config)
}

func TestInstallRelease_NoName(t *testing.T) {
	instAction := installAction(t)
	instAction.ReleaseName = ""
	vals := map[string]any{}
	_, err := instAction.Run(buildChart(), vals)
	assert.ErrorContains(t, err, "no name provided")
}

func TestInstallRelease_WithNotes(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(withNotes("note here")), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	is.Equal("with-notes", res.Name)
	is.Equal("spaced", res.Namespace)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)
	rel, err := releaserToV1Release(r)
	req.NoError(err)
	is.Len(rel.Hooks, 1)
	is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
	is.Equal(release.HookPostInstall, rel.Hooks[0].Events[0])
	is.Equal(release.HookPreDelete, rel.Hooks[0].Events[1], "Expected event 0 is pre-delete")
	is.NotEmpty(res.Manifest)
	is.NotEmpty(rel.Manifest)
	is.Contains(rel.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
	is.Equal("Install complete", rel.Info.Description)

	is.Equal("note here", rel.Info.Notes)
}

func TestInstallRelease_WithNotesRendered(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(withNotes("got-{{.Release.Name}}")), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)
	rel, err := releaserToV1Release(r)
	req.NoError(err)

	expectedNotes := "got-" + res.Name
	is.Equal(expectedNotes, rel.Info.Notes)
	is.Equal("Install complete", rel.Info.Description)
}

func TestInstallRelease_WithChartAndDependencyParentNotes(t *testing.T) {
	// Regression: Make sure that the child's notes don't override the parent's
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(withNotes("parent"), withDependency(withNotes("child"))), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)
	rel, err := releaserToV1Release(r)
	req.NoError(err)
	is.Equal("with-notes", rel.Name)
	is.Equal("parent", rel.Info.Notes)
	is.Equal("Install complete", rel.Info.Description)
}

func TestInstallRelease_WithChartAndDependencyAllNotes(t *testing.T) {
	// Regression: Make sure that the child's notes don't override the parent's
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "with-notes"
	instAction.SubNotes = true
	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(withNotes("parent"), withDependency(withNotes("child"))), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)
	rel, err := releaserToV1Release(r)
	req.NoError(err)
	is.Equal("with-notes", rel.Name)
	// test run can return as either 'parent\nchild' or 'child\nparent'
	req.True(strings.Contains(rel.Info.Notes, "parent") || strings.Contains(rel.Info.Notes, "child"), "Expected 'parent\nchild' or 'child\nparent', got '%s'", rel.Info.Notes)
	is.Equal("Install complete", rel.Info.Description)
}

func TestInstallRelease_DryRunClient(t *testing.T) {
	for _, dryRunStrategy := range []DryRunStrategy{DryRunClient, DryRunServer} {
		is := assert.New(t)
		req := require.New(t)
		instAction := installAction(t)
		instAction.DryRunStrategy = dryRunStrategy

		vals := map[string]any{}
		resi, err := instAction.Run(buildChart(withSampleTemplates()), vals)
		req.NoError(err, "Failed install")
		res, err := releaserToV1Release(resi)
		req.NoError(err)

		is.Contains(res.Manifest, "---\n# Source: hello/templates/hello\nhello: world")
		is.Contains(res.Manifest, "---\n# Source: hello/templates/goodbye\ngoodbye: world")
		is.Contains(res.Manifest, "hello: Earth")
		is.NotContains(res.Manifest, "hello: {{ template \"_planet\" . }}")
		is.NotContains(res.Manifest, "empty")

		_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
		req.Error(err)
		is.Len(res.Hooks, 1)
		is.Zero(res.Hooks[0].LastRun.CompletedAt, "expect hook to not be marked as run")
		is.Equal("Dry run complete", res.Info.Description)
	}
}

func TestInstallRelease_DryRunHiddenSecret(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)

	// First perform a normal dry-run with the secret and confirm its presence.
	instAction.DryRunStrategy = DryRunClient
	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Contains(res.Manifest, "---\n# Source: hello/templates/secret.yaml\napiVersion: v1\nkind: Secret")

	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	req.Error(err)
	is.Equal("Dry run complete", res.Info.Description)

	// Perform a dry-run where the secret should not be present
	instAction.HideSecret = true
	vals = map[string]any{}
	res2i, err := instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	req.NoError(err, "Failed install")
	res2, err := releaserToV1Release(res2i)
	req.NoError(err)

	is.NotContains(res2.Manifest, "---\n# Source: hello/templates/secret.yaml\napiVersion: v1\nkind: Secret")

	_, err = instAction.cfg.Releases.Get(res2.Name, res2.Version)
	req.Error(err)
	is.Equal("Dry run complete", res2.Info.Description)

	// Ensure there is an error when HideSecret True but not in a dry-run mode
	instAction.DryRunStrategy = DryRunNone
	vals = map[string]any{}
	_, err = instAction.Run(buildChart(withSampleSecret(), withSampleTemplates()), vals)
	req.Error(err, "Did not get the expected error when dry-run is false and hide secret is true")
}

// Regression test for #7955
func TestInstallRelease_DryRun_Lookup(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.DryRunStrategy = DryRunNone
	vals := map[string]any{}

	mockChart := buildChart(withSampleTemplates())
	mockChart.Templates = append(mockChart.Templates, &common.File{
		Name:    "templates/lookup",
		ModTime: time.Now(),
		Data:    []byte(`goodbye: {{ lookup "v1" "Namespace" "" "___" }}`),
	})

	resi, err := instAction.Run(mockChart, vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	is.Contains(res.Manifest, "goodbye: map[]")
}

func TestInstallReleaseIncorrectTemplate_DryRun(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.DryRunStrategy = DryRunNone
	vals := map[string]any{}
	_, err := instAction.Run(buildChart(withSampleIncludingIncorrectTemplates()), vals)
	expectedErr := `hello/templates/incorrect:1:10
  executing "hello/templates/incorrect" at <.Values.bad.doh>:
    nil pointer evaluating interface {}.doh`
	req.Error(err, "Install should fail containing error: %s", expectedErr)
	is.ErrorContains(err, expectedErr)
}

func TestInstallRelease_NoHooks(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.DisableHooks = true
	instAction.ReleaseName = "no-hooks"
	req.NoError(instAction.cfg.Releases.Create(releaseStub()))

	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(), vals)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	is.Zero(res.Hooks[0].LastRun.CompletedAt, "hooks should not run with no-hooks")
}

func TestInstallRelease_FailedHooks(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "failed-hooks"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WatchUntilReadyError = errors.New("Failed watch")
	instAction.cfg.KubeClient = failer
	outBuffer := &bytes.Buffer{}
	failer.PrintingKubeClient = kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: outBuffer}

	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(), vals)
	req.Error(err)
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Contains(res.Info.Description, "failed post-install")
	is.Empty(outBuffer.String())
	is.Equal(rcommon.StatusFailed, res.Info.Status)
}

func TestInstallRelease_ReplaceRelease(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.Replace = true

	rel := releaseStub()
	rel.Info.Status = rcommon.StatusUninstalled
	req.NoError(instAction.cfg.Releases.Create(rel))
	instAction.ReleaseName = rel.Name

	vals := map[string]any{}
	resi, err := instAction.Run(buildChart(), vals)
	req.NoError(err)
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	// This should have been auto-incremented
	is.Equal(2, res.Version)
	is.Equal(res.Name, rel.Name)

	r, err := instAction.cfg.Releases.Get(rel.Name, res.Version)
	req.NoError(err)
	getres, err := releaserToV1Release(r)
	req.NoError(err)
	is.Equal(rcommon.StatusDeployed, getres.Info.Status)
}

func TestInstallRelease_KubeVersion(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	vals := map[string]any{}
	_, err := instAction.Run(buildChart(withKube(">=0.0.0")), vals)
	req.NoError(err)

	// This should fail for a few hundred years
	instAction.ReleaseName = "should-fail"
	vals = map[string]any{}
	_, err = instAction.Run(buildChart(withKube(">=99.0.0")), vals)
	req.Error(err)
	is.ErrorContains(err, "chart requires kubeVersion: >=99.0.0 which is incompatible with Kubernetes v1.20.")
}

func TestInstallRelease_Wait(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "come-fail-away"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = errors.New("I timed out")
	instAction.cfg.KubeClient = failer
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]any{}

	goroutines := instAction.getGoroutineCount()

	resi, err := instAction.Run(buildChart(), vals)
	req.Error(err)
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(rcommon.StatusFailed, res.Info.Status)

	is.Equal(goroutines, instAction.getGoroutineCount())
}
func TestInstallRelease_Wait_Interrupted(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]any{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	goroutines := instAction.getGoroutineCount()

	_, err := instAction.RunWithContext(ctx, buildChart(), vals)
	req.Error(err)
	req.ErrorContains(err, "context canceled")

	is.Equal(goroutines+1, instAction.getGoroutineCount()) // installation goroutine still is in background
	time.Sleep(10 * time.Second)                           // wait for goroutine to finish
	is.Equal(goroutines, instAction.getGoroutineCount())
}
func TestInstallRelease_WaitForJobs(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "come-fail-away"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = errors.New("I timed out")
	instAction.cfg.KubeClient = failer
	instAction.WaitStrategy = kube.StatusWatcherStrategy
	instAction.WaitForJobs = true
	vals := map[string]any{}

	resi, err := instAction.Run(buildChart(), vals)
	req.Error(err)
	res, err := releaserToV1Release(resi)
	req.NoError(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(rcommon.StatusFailed, res.Info.Status)
}

func TestInstallRelease_RollbackOnFailure(t *testing.T) {
	t.Run("rollback-on-failure uninstall succeeds", func(t *testing.T) {
		is := assert.New(t)
		req := require.New(t)
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = errors.New("I timed out")
		instAction.cfg.KubeClient = failer
		instAction.RollbackOnFailure = true
		// disabling hooks to avoid an early fail when
		// WaitForDelete is called on the pre-delete hook execution
		instAction.DisableHooks = true
		vals := map[string]any{}

		resi, err := instAction.Run(buildChart(), vals)
		req.Error(err)
		req.ErrorContains(err, "I timed out")
		req.ErrorContains(err, "rollback-on-failure")

		res, err := releaserToV1Release(resi)
		req.NoError(err)
		// Now make sure it isn't in storage anymore
		_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
		req.Error(err)
		is.Equal(err, driver.ErrReleaseNotFound)
	})

	t.Run("rollback-on-failure uninstall fails", func(t *testing.T) {
		is := assert.New(t)
		req := require.New(t)
		instAction := installAction(t)
		instAction.ReleaseName = "come-fail-away-with-me"
		failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.WaitError = errors.New("I timed out")
		failer.DeleteError = errors.New("uninstall fail")
		instAction.cfg.KubeClient = failer
		instAction.RollbackOnFailure = true
		vals := map[string]any{}

		_, err := instAction.Run(buildChart(), vals)
		req.Error(err)
		req.ErrorContains(err, "I timed out")
		req.ErrorContains(err, "uninstall fail")
		is.ErrorContains(err, "an error occurred while uninstalling the release")
	})
}
func TestInstallRelease_RollbackOnFailure_Interrupted(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.ReleaseName = "interrupted-release"
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	instAction.cfg.KubeClient = failer
	instAction.RollbackOnFailure = true
	vals := map[string]any{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	goroutines := instAction.getGoroutineCount()

	resi, err := instAction.RunWithContext(ctx, buildChart(), vals)
	req.Error(err)
	req.ErrorContains(err, "context canceled")
	req.ErrorContains(err, "rollback-on-failure")
	req.ErrorContains(err, "uninstalled")

	res, err := releaserToV1Release(resi)
	req.NoError(err)
	// Now make sure it isn't in storage anymore
	_, err = instAction.cfg.Releases.Get(res.Name, res.Version)
	req.Error(err)
	is.Equal(err, driver.ErrReleaseNotFound)
	is.Equal(goroutines+1, instAction.getGoroutineCount()) // installation goroutine still is in background
	time.Sleep(10 * time.Second)                           // wait for goroutine to finish
	is.Equal(goroutines, instAction.getGoroutineCount())
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
		t.Run(tc.tpl, func(t *testing.T) {
			n, err := TemplateName(tc.tpl)
			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				re, compErr := regexp.Compile(tc.expectedErrorStr)
				require.NoError(t, compErr, "Expected error string failed to compile")
				assert.True(t, re.MatchString(err.Error()), "Error didn't match for %s expected %s", tc.tpl, tc.expectedErrorStr)
			}

			if tc.expected != "" {
				re, err := regexp.Compile(tc.expected)
				require.NoError(t, err)
				assert.True(t, re.MatchString(n), "Returned name didn't match for %s expected %s but got %s", tc.tpl, tc.expected, n)
			}
		})
	}
}

func TestInstallReleaseOutputDir(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	vals := map[string]any{}

	dir := t.TempDir()

	instAction.OutputDir = dir

	_, err := instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
	req.NoError(err, "Failed install")

	_, err = os.Stat(filepath.Join(dir, "hello", "templates", "goodbye"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello", "templates", "hello"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello", "templates", "with-partials"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(dir, "hello", "templates", "rbac"))
	req.NoError(err)

	test.AssertGoldenFile(t, filepath.Join(dir, "hello", "templates", "rbac"), "rbac.txt")

	_, err = os.Stat(filepath.Join(dir, "hello", "templates", "empty"))
	is.ErrorIs(err, fs.ErrNotExist)
}

func TestInstallOutputDirWithReleaseName(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	vals := map[string]any{}

	dir := t.TempDir()

	instAction.OutputDir = dir
	instAction.UseReleaseName = true
	instAction.ReleaseName = "madra"

	newDir := filepath.Join(dir, instAction.ReleaseName)

	_, err := instAction.Run(buildChart(withSampleTemplates(), withMultipleManifestTemplate()), vals)
	req.NoError(err, "Failed install")

	_, err = os.Stat(filepath.Join(newDir, "hello", "templates", "goodbye"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello", "templates", "hello"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello", "templates", "with-partials"))
	req.NoError(err)

	_, err = os.Stat(filepath.Join(newDir, "hello", "templates", "rbac"))
	req.NoError(err)

	test.AssertGoldenFile(t, filepath.Join(newDir, "hello", "templates", "rbac"), "rbac.txt")

	_, err = os.Stat(filepath.Join(newDir, "hello", "templates", "empty"))
	is.ErrorIs(err, fs.ErrNotExist)
}

func TestNameAndChart(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	chartName := "./foo"

	name, chrt, err := instAction.NameAndChart([]string{chartName})
	req.NoError(err)
	is.Equal(instAction.ReleaseName, name)
	is.Equal(chartName, chrt)

	instAction.GenerateName = true
	_, _, err = instAction.NameAndChart([]string{"foo", chartName})
	req.Error(err, "expected an error")
	req.EqualError(err, "cannot set --generate-name and also specify a name")

	instAction.GenerateName = false
	instAction.NameTemplate = "{{ . }}"
	_, _, err = instAction.NameAndChart([]string{"foo", chartName})
	req.Error(err, "expected an error")
	req.EqualError(err, "cannot set --name-template and also specify a name")

	instAction.NameTemplate = ""
	instAction.ReleaseName = ""
	_, _, err = instAction.NameAndChart([]string{chartName})
	req.Error(err, "expected an error")
	req.EqualError(err, "must either provide a name or specify --generate-name")

	instAction.NameTemplate = ""
	instAction.ReleaseName = ""
	_, _, err = instAction.NameAndChart([]string{"foo", chartName, "bar"})
	req.Error(err, "expected an error")
	is.EqualError(err, "expected at most two arguments, unexpected arguments: bar")
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
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
		{
			"dot filepath",
			".",
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
		{
			"empty filepath",
			"",
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
		{
			"packaged chart",
			"chart.tgz",
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
		{
			"packaged chart with .tar.gz extension",
			"chart.tar.gz",
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
		{
			"packaged chart with local extension",
			"./chart.tgz",
			fmt.Sprintf("chart-%d", time.Now().Unix()),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			name, chrt, err := instAction.NameAndChart([]string{tc.Chart})
			require.NoError(t, err)

			is.Equal(tc.ExpectedName, name)
			is.Equal(tc.Chart, chrt)
		})
	}
}

func TestInstallWithLabels(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.Labels = map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	resi, err := instAction.Run(buildChart(), nil)
	req.NoError(err, "Failed install")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	is.Equal(instAction.Labels, res.Labels)
}

func TestInstallWithSystemLabels(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
	instAction := installAction(t)
	instAction.Labels = map[string]string{
		"owner": "val1",
		"key2":  "val2",
	}
	_, err := instAction.Run(buildChart(), nil)
	req.Error(err, "expected an error")
	is.EqualError(err, fmt.Sprintf("user supplied labels contains system reserved label name. System labels: %+v", driver.GetSystemLabels()))
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
			require.NoError(t, err, "Failed to parse URL1 %s", tc.url1)
			u2, err := url.Parse(tc.url2)
			require.NoError(t, err, "Failed to parse URL2 %s", tc.url2)

			is.Equal(tc.expected, urlEqual(u1, u2))
		})
	}
}

func TestInstallRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	instAction := NewInstall(config)
	ctx, done := context.WithCancel(t.Context())
	chrt := buildChart()
	res, err := instAction.RunWithContext(ctx, chrt, nil)

	done()
	assert.Nil(t, res)
	assert.ErrorContains(t, err, "connection refused")
}

func TestInstallSetRegistryClient(t *testing.T) {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)

	registryClient := &registry.Client{}
	instAction.SetRegistryClient(registryClient)

	assert.Equal(t, registryClient, instAction.GetRegistryClient())
}

func TestInstallCRDs(t *testing.T) {
	config := actionConfigFixtureWithDummyResources(t, createDummyCRDList(false))
	instAction := NewInstall(config)

	mockFile := common.File{
		Name: "crds/foo.yaml",
		Data: []byte("hello"),
	}
	mockChart := buildChart(withFile(mockFile))
	crdsToInstall := mockChart.CRDObjects()

	require.Len(t, crdsToInstall, 1)
	assert.Equal(t, crdsToInstall[0].File.Data, mockFile.Data)
	require.NoError(t, instAction.installCRDs(crdsToInstall))
}

func TestInstallCRDs_AlreadyExist(t *testing.T) {
	dummyResources := createDummyCRDList(false)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: dummyResources}
	mockError := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusFailure,
		Reason: metav1.StatusReasonAlreadyExists,
	}}
	failingKubeClient.CreateError = mockError

	config := actionConfigFixtureWithDummyResources(t, dummyResources)
	config.KubeClient = &failingKubeClient
	instAction := NewInstall(config)

	mockFile := common.File{
		Name: "crds/foo.yaml",
		Data: []byte("hello"),
	}
	mockChart := buildChart(withFile(mockFile))
	crdsToInstall := mockChart.CRDObjects()

	assert.NoError(t, instAction.installCRDs(crdsToInstall))
}

func TestInstallCRDs_KubeClient_BuildError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.BuildError = errors.New("build error")
	config.KubeClient = &failingKubeClient
	instAction := NewInstall(config)

	mockFile := common.File{
		Name: "crds/foo.yaml",
		Data: []byte("hello"),
	}
	mockChart := buildChart(withFile(mockFile))
	crdsToInstall := mockChart.CRDObjects()

	require.Error(t, instAction.installCRDs(crdsToInstall), "failed to install CRD")
}

func TestInstallCRDs_KubeClient_CreateError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.CreateError = errors.New("create error")
	config.KubeClient = &failingKubeClient
	instAction := NewInstall(config)

	mockFile := common.File{
		Name: "crds/foo.yaml",
		Data: []byte("hello"),
	}
	mockChart := buildChart(withFile(mockFile))
	crdsToInstall := mockChart.CRDObjects()

	require.Error(t, instAction.installCRDs(crdsToInstall), "failed to install CRD")
}

func TestInstallCRDs_WaiterError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.WaitError = errors.New("wait error")
	failingKubeClient.BuildDummy = true
	config.KubeClient = &failingKubeClient
	instAction := NewInstall(config)

	mockFile := common.File{
		Name: "crds/foo.yaml",
		Data: []byte("hello"),
	}
	mockChart := buildChart(withFile(mockFile))
	crdsToInstall := mockChart.CRDObjects()

	require.Error(t, instAction.installCRDs(crdsToInstall), "wait error")
}

func TestCheckDependencies(t *testing.T) {
	dependency := chart.Dependency{Name: "hello"}
	mockChart := buildChart(withDependency())

	assert.NoError(t, CheckDependencies(mockChart, []ci.Dependency{&dependency}))
}

func TestCheckDependencies_MissingDependency(t *testing.T) {
	dependency := chart.Dependency{Name: "missing"}
	mockChart := buildChart(withDependency())

	assert.ErrorContains(t, CheckDependencies(mockChart, []ci.Dependency{&dependency}), "missing in charts")
}

func TestInstallCRDs_CheckNilErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []chart.CRD
	}{
		{
			name: "only one crd with file nil",
			input: []chart.CRD{
				{Name: "one", File: nil},
			},
		},
		{
			name: "only one crd with its file data nil",
			input: []chart.CRD{
				{Name: "one", File: &common.File{Name: "crds/foo.yaml", Data: nil}},
			},
		},
		{
			name: "at least a crd with its file data nil",
			input: []chart.CRD{
				{Name: "one", File: &common.File{Name: "crds/foo.yaml", Data: []byte("data")}},
				{Name: "two", File: &common.File{Name: "crds/foo2.yaml", Data: nil}},
				{Name: "three", File: &common.File{Name: "crds/foo3.yaml", Data: []byte("data")}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instAction := installAction(t)
			assert.Error(t, instAction.installCRDs(tt.input), "got nil expected err")
		})
	}
}

func TestInstallRelease_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	instAction := installAction(t)
	instAction.ReleaseName = "wait-options-test"
	instAction.WaitStrategy = kube.StatusWatcherStrategy

	// Use WithWaitContext as a marker WaitOption that we can track
	ctx := context.Background()
	instAction.WaitOptions = []kube.WaitOption{kube.WithWaitContext(ctx)}

	// Access the underlying FailingKubeClient to check recorded options
	failer := instAction.cfg.KubeClient.(*kubefake.FailingKubeClient)

	vals := map[string]any{}
	_, err := instAction.Run(buildChart(), vals)
	req.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}
