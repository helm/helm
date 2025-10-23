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
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"helm.sh/helm/v4/internal/logging"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/registry"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

var verbose = flag.Bool("test.log", false, "enable test logging (debug by default)")

func actionConfigFixture(t *testing.T) *Configuration {
	t.Helper()
	return actionConfigFixtureWithDummyResources(t, nil)
}

func actionConfigFixtureWithDummyResources(t *testing.T, dummyResources kube.ResourceList) *Configuration {
	t.Helper()

	logger := logging.NewLogger(func() bool {
		return *verbose
	})
	slog.SetDefault(logger)

	registryClient, err := registry.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	return &Configuration{
		Releases:       storage.Init(driver.NewMemory()),
		KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: dummyResources},
		Capabilities:   common.DefaultCapabilities,
		RegistryClient: registryClient,
	}
}

var manifestWithHook = `kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete,post-upgrade
data:
  name: value`

var manifestWithTestHook = `kind: Pod
  metadata:
	name: finding-nemo,
	annotations:
	  "helm.sh/hook": test
  spec:
	containers:
	- name: nemo-test
	  image: fake-image
	  cmd: fake-command
  `

var rbacManifests = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: schedule-agents
rules:
- apiGroups: [""]
  resources: ["pods", "pods/exec", "pods/log"]
  verbs: ["*"]

---

apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: schedule-agents
  namespace: {{ default .Release.Namespace}}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: schedule-agents
subjects:
- kind: ServiceAccount
  name: schedule-agents
  namespace: {{ .Release.Namespace }}
`

type chartOptions struct {
	*chart.Chart
}

type chartOption func(*chartOptions)

func buildChart(opts ...chartOption) *chart.Chart {
	defaultTemplates := []*common.File{
		{Name: "templates/hello", Data: []byte("hello: world")},
		{Name: "templates/hooks", Data: []byte(manifestWithHook)},
	}
	return buildChartWithTemplates(defaultTemplates, opts...)
}

func buildChartWithTemplates(templates []*common.File, opts ...chartOption) *chart.Chart {
	c := &chartOptions{
		Chart: &chart.Chart{
			// TODO: This should be more complete.
			Metadata: &chart.Metadata{
				APIVersion: "v1",
				Name:       "hello",
				Version:    "0.1.0",
			},
			Templates: templates,
		},
	}

	for _, opt := range opts {
		opt(c)
	}
	return c.Chart
}

func withName(name string) chartOption {
	return func(opts *chartOptions) {
		opts.Metadata.Name = name
	}
}

func withSampleValues() chartOption {
	values := map[string]interface{}{
		"someKey": "someValue",
		"nestedKey": map[string]interface{}{
			"simpleKey": "simpleValue",
			"anotherNestedKey": map[string]interface{}{
				"yetAnotherNestedKey": map[string]interface{}{
					"youReadyForAnotherNestedKey": "No",
				},
			},
		},
	}
	return func(opts *chartOptions) {
		opts.Values = values
	}
}

func withValues(values map[string]interface{}) chartOption {
	return func(opts *chartOptions) {
		opts.Values = values
	}
}

func withNotes(notes string) chartOption {
	return func(opts *chartOptions) {
		opts.Templates = append(opts.Templates, &common.File{
			Name: "templates/NOTES.txt",
			Data: []byte(notes),
		})
	}
}

func withDependency(dependencyOpts ...chartOption) chartOption {
	return func(opts *chartOptions) {
		opts.AddDependency(buildChart(dependencyOpts...))
	}
}

func withMetadataDependency(dependency chart.Dependency) chartOption {
	return func(opts *chartOptions) {
		opts.Metadata.Dependencies = append(opts.Metadata.Dependencies, &dependency)
	}
}

func withSampleTemplates() chartOption {
	return func(opts *chartOptions) {
		sampleTemplates := []*common.File{
			// This adds basic templates and partials.
			{Name: "templates/goodbye", Data: []byte("goodbye: world")},
			{Name: "templates/empty", Data: []byte("")},
			{Name: "templates/with-partials", Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
		}
		opts.Templates = append(opts.Templates, sampleTemplates...)
	}
}

func withSampleSecret() chartOption {
	return func(opts *chartOptions) {
		sampleSecret := &common.File{Name: "templates/secret.yaml", Data: []byte("apiVersion: v1\nkind: Secret\n")}
		opts.Templates = append(opts.Templates, sampleSecret)
	}
}

func withSampleIncludingIncorrectTemplates() chartOption {
	return func(opts *chartOptions) {
		sampleTemplates := []*common.File{
			// This adds basic templates and partials.
			{Name: "templates/goodbye", Data: []byte("goodbye: world")},
			{Name: "templates/empty", Data: []byte("")},
			{Name: "templates/incorrect", Data: []byte("{{ .Values.bad.doh }}")},
			{Name: "templates/with-partials", Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
		}
		opts.Templates = append(opts.Templates, sampleTemplates...)
	}
}

func withMultipleManifestTemplate() chartOption {
	return func(opts *chartOptions) {
		sampleTemplates := []*common.File{
			{Name: "templates/rbac", Data: []byte(rbacManifests)},
		}
		opts.Templates = append(opts.Templates, sampleTemplates...)
	}
}

func withKube(version string) chartOption {
	return func(opts *chartOptions) {
		opts.Metadata.KubeVersion = version
	}
}

// releaseStub creates a release stub, complete with the chartStub as its chart.
func releaseStub() *release.Release {
	return namedReleaseStub("angry-panda", rcommon.StatusDeployed)
}

func namedReleaseStub(name string, status rcommon.Status) *release.Release {
	now := time.Now()
	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        status,
			Description:   "Named Release Stub",
		},
		Chart:   buildChart(withSampleTemplates()),
		Config:  map[string]interface{}{"name": "value"},
		Version: 1,
		Hooks: []*release.Hook{
			{
				Name:     "test-cm",
				Kind:     "ConfigMap",
				Path:     "test-cm",
				Manifest: manifestWithHook,
				Events: []release.HookEvent{
					release.HookPostInstall,
					release.HookPreDelete,
				},
			},
			{
				Name:     "finding-nemo",
				Kind:     "Pod",
				Path:     "finding-nemo",
				Manifest: manifestWithTestHook,
				Events: []release.HookEvent{
					release.HookTest,
				},
			},
		},
	}
}

func TestConfiguration_Init(t *testing.T) {
	tests := []struct {
		name               string
		helmDriver         string
		expectedDriverType interface{}
		expectErr          bool
		errMsg             string
	}{
		{
			name:               "Test secret driver",
			helmDriver:         "secret",
			expectedDriverType: &driver.Secrets{},
		},
		{
			name:               "Test secrets driver",
			helmDriver:         "secrets",
			expectedDriverType: &driver.Secrets{},
		},
		{
			name:               "Test empty driver",
			helmDriver:         "",
			expectedDriverType: &driver.Secrets{},
		},
		{
			name:               "Test configmap driver",
			helmDriver:         "configmap",
			expectedDriverType: &driver.ConfigMaps{},
		},
		{
			name:               "Test configmaps driver",
			helmDriver:         "configmaps",
			expectedDriverType: &driver.ConfigMaps{},
		},
		{
			name:               "Test memory driver",
			helmDriver:         "memory",
			expectedDriverType: &driver.Memory{},
		},
		{
			name:       "Test sql driver",
			helmDriver: "sql",
			expectErr:  true,
			errMsg:     "unable to instantiate SQL driver",
		},
		{
			name:       "Test unknown driver",
			helmDriver: "someDriver",
			expectErr:  true,
			errMsg:     fmt.Sprintf("unknown driver %q", "someDriver"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Configuration{}

			actualErr := cfg.Init(nil, "default", tt.helmDriver)
			if tt.expectErr {
				assert.Error(t, actualErr)
				assert.Contains(t, actualErr.Error(), tt.errMsg)
			} else {
				assert.NoError(t, actualErr)
				assert.IsType(t, tt.expectedDriverType, cfg.Releases.Driver)
			}
		})
	}
}

func TestGetVersionSet(t *testing.T) {
	client := fakeclientset.NewClientset()

	vs, err := GetVersionSet(client.Discovery())
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

// Mock PostRenderer for testing
type mockPostRenderer struct {
	shouldError bool
	transform   func(string) string
}

func (m *mockPostRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	if m.shouldError {
		return nil, errors.New("mock post-renderer error")
	}

	content := renderedManifests.String()
	if m.transform != nil {
		content = m.transform(content)
	}

	return bytes.NewBufferString(content), nil
}

func TestAnnotateAndMerge(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		expectedError string
		expected      string
	}{
		{
			name:     "no files",
			files:    map[string]string{},
			expected: "",
		},
		{
			name: "single file with single manifest",
			files: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/configmap.yaml'
data:
  key: value
`,
		},
		{
			name: "multiple files with multiple manifests",
			files: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`,
				"templates/secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: dGVzdA==`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/configmap.yaml'
data:
  key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/secret.yaml'
data:
  password: dGVzdA==
`,
		},
		{
			name: "file with multiple manifests",
			files: map[string]string{
				"templates/multi.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm2
data:
  key: value2`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/multi.yaml'
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/multi.yaml'
data:
  key: value2
`,
		},
		{
			name: "partials and empty files are removed",
			files: map[string]string{
				"templates/cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
`,
				"templates/_partial.tpl": `
{{-define name}}
  {{- "abracadabra"}}
{{- end -}}`,
				"templates/empty.yaml": ``,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
`,
		},
		{
			name: "empty file",
			files: map[string]string{
				"templates/empty.yaml": "",
			},
			expected: ``,
		},
		{
			name: "invalid yaml",
			files: map[string]string{
				"templates/invalid.yaml": `invalid: yaml: content:
  - malformed`,
			},
			expectedError: "parsing templates/invalid.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := annotateAndMerge(tt.files)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, merged)
				assert.Equal(t, tt.expected, merged)
			}
		})
	}
}

func TestSplitAndDeannotate(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedFiles map[string]string
		expectedError string
	}{
		{
			name: "single annotated manifest",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/configmap.yaml
data:
  key: value`,
			expectedFiles: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`,
			},
		},
		{
			name: "multiple manifests with different filenames",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/configmap.yaml
data:
  key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/secret.yaml
data:
  password: dGVzdA==`,
			expectedFiles: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`,
				"templates/secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: dGVzdA==
`,
			},
		},
		{
			name: "multiple manifests with same filename",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/multi.yaml
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm2
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/multi.yaml
data:
  key: value2`,
			expectedFiles: map[string]string{
				"templates/multi.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm2
data:
  key: value2
`,
			},
		},
		{
			name: "manifest with other annotations",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: templates/configmap.yaml
    other-annotation: should-remain
data:
  key: value`,
			expectedFiles: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    other-annotation: should-remain
data:
  key: value
`,
			},
		},
		{
			name:          "invalid yaml input",
			input:         "invalid: yaml: content:",
			expectedError: "error parsing YAML: MalformedYAMLError",
		},
		{
			name: "manifest without filename annotation",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`,
			expectedFiles: map[string]string{
				"generated-by-postrender-0.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := splitAndDeannotate(tt.input)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedFiles), len(files))

				for expectedFile, expectedContent := range tt.expectedFiles {
					actualContent, exists := files[expectedFile]
					assert.True(t, exists, "Expected file %s not found", expectedFile)
					assert.Equal(t, expectedContent, actualContent)
				}
			}
		})
	}
}

func TestAnnotateAndMerge_SplitAndDeannotate_Roundtrip(t *testing.T) {
	// Test that merge/split operations are symmetric
	originalFiles := map[string]string{
		"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`,
		"templates/secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  password: dGVzdA==`,
		"templates/multi.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm1
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm2
data:
  key: value2`,
	}

	// Merge and annotate
	merged, err := annotateAndMerge(originalFiles)
	require.NoError(t, err)

	// Split and deannotate
	reconstructed, err := splitAndDeannotate(merged)
	require.NoError(t, err)

	// Compare the results
	assert.Equal(t, len(originalFiles), len(reconstructed))
	for filename, originalContent := range originalFiles {
		reconstructedContent, exists := reconstructed[filename]
		assert.True(t, exists, "File %s should exist in reconstructed files", filename)

		// Normalize whitespace for comparison since YAML processing might affect formatting
		normalizeContent := func(content string) string {
			return strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
		}

		assert.Equal(t, normalizeContent(originalContent), normalizeContent(reconstructedContent))
	}
}

func TestRenderResources_PostRenderer_Success(t *testing.T) {
	cfg := actionConfigFixture(t)

	// Create a simple mock post-renderer
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			content = strings.ReplaceAll(content, "hello", "yellow")
			content = strings.ReplaceAll(content, "goodbye", "foodpie")
			return strings.ReplaceAll(content, "test-cm", "test-cm-postrendered")
		},
	}

	ch := buildChart(withSampleTemplates())
	values := map[string]interface{}{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, false,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Equal(t, "", notes)
	expectedBuf := `---
# Source: yellow/templates/foodpie
foodpie: world
---
# Source: yellow/templates/with-partials
yellow: Earth
---
# Source: yellow/templates/yellow
yellow: world
`
	expectedHook := `kind: ConfigMap
metadata:
  name: test-cm-postrendered
  annotations:
    "helm.sh/hook": post-install,pre-delete,post-upgrade
data:
  name: value`

	assert.Equal(t, expectedBuf, buf.String())
	assert.Len(t, hooks, 1)
	assert.Equal(t, expectedHook, hooks[0].Manifest)
}

func TestRenderResources_PostRenderer_Error(t *testing.T) {
	cfg := actionConfigFixture(t)

	// Create a post-renderer that returns an error
	mockPR := &mockPostRenderer{
		shouldError: true,
	}

	ch := buildChart(withSampleTemplates())
	values := map[string]interface{}{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, false,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error while running post render on files")
}

func TestRenderResources_PostRenderer_MergeError(t *testing.T) {
	cfg := actionConfigFixture(t)

	// Create a mock post-renderer
	mockPR := &mockPostRenderer{}

	// Create a chart with invalid YAML that would cause AnnotateAndMerge to fail
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v1",
			Name:       "test-chart",
			Version:    "0.1.0",
		},
		Templates: []*common.File{
			{Name: "templates/invalid", Data: []byte("invalid: yaml: content:")},
		},
	}
	values := map[string]interface{}{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, false,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error merging manifests")
}

func TestRenderResources_PostRenderer_SplitError(t *testing.T) {
	cfg := actionConfigFixture(t)

	// Create a post-renderer that returns invalid YAML
	mockPR := &mockPostRenderer{
		transform: func(_ string) string {
			return "invalid: yaml: content:"
		},
	}

	ch := buildChart(withSampleTemplates())
	values := map[string]interface{}{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, false,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error while parsing post rendered output: error parsing YAML: MalformedYAMLError:")
}

func TestRenderResources_PostRenderer_Integration(t *testing.T) {
	cfg := actionConfigFixture(t)

	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			return strings.ReplaceAll(content, "metadata:", "color: blue\nmetadata:")
		},
	}

	ch := buildChart(withSampleTemplates())
	values := map[string]interface{}{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, false,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Equal(t, "", notes) // Notes should be empty for this test

	// Verify that the post-renderer modifications are present in the output
	output := buf.String()
	expected := `---
# Source: hello/templates/goodbye
goodbye: world
color: blue
---
# Source: hello/templates/hello
hello: world
color: blue
---
# Source: hello/templates/with-partials
hello: Earth
color: blue
`
	assert.Contains(t, output, "color: blue")
	assert.Equal(t, 3, strings.Count(output, "color: blue"))
	assert.Equal(t, expected, output)
}

func TestRenderResources_NoPostRenderer(t *testing.T) {
	cfg := actionConfigFixture(t)

	ch := buildChart(withSampleTemplates())
	values := map[string]interface{}{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		nil, false, false, false, false,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Equal(t, "", notes)
}

func TestDetermineReleaseSSAApplyMethod(t *testing.T) {
	assert.Equal(t, release.ApplyMethodClientSideApply, determineReleaseSSApplyMethod(false))
	assert.Equal(t, release.ApplyMethodServerSideApply, determineReleaseSSApplyMethod(true))
}

func TestIsDryRun(t *testing.T) {
	assert.False(t, isDryRun(DryRunNone))
	assert.True(t, isDryRun(DryRunClient))
	assert.True(t, isDryRun(DryRunServer))
}

func TestInteractWithServer(t *testing.T) {
	assert.True(t, interactWithServer(DryRunNone))
	assert.False(t, interactWithServer(DryRunClient))
	assert.True(t, interactWithServer(DryRunServer))
}
