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
	modTime := time.Now()
	defaultTemplates := []*common.File{
		{Name: "templates/hello", ModTime: modTime, Data: []byte("hello: world")},
		{Name: "templates/hooks", ModTime: modTime, Data: []byte(manifestWithHook)},
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
	values := map[string]any{
		"someKey": "someValue",
		"nestedKey": map[string]any{
			"simpleKey": "simpleValue",
			"anotherNestedKey": map[string]any{
				"yetAnotherNestedKey": map[string]any{
					"youReadyForAnotherNestedKey": "No",
				},
			},
		},
	}
	return func(opts *chartOptions) {
		opts.Values = values
	}
}

func withValues(values map[string]any) chartOption {
	return func(opts *chartOptions) {
		opts.Values = values
	}
}

func withNotes(notes string) chartOption {
	return func(opts *chartOptions) {
		opts.Templates = append(opts.Templates, &common.File{
			Name:    "templates/NOTES.txt",
			ModTime: time.Now(),
			Data:    []byte(notes),
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

func withFile(file common.File) chartOption {
	return func(opts *chartOptions) {
		opts.Files = append(opts.Files, &file)
	}
}

func withSampleTemplates() chartOption {
	return func(opts *chartOptions) {
		modTime := time.Now()
		sampleTemplates := []*common.File{
			// This adds basic templates and partials.
			{Name: "templates/goodbye", ModTime: modTime, Data: []byte("goodbye: world")},
			{Name: "templates/empty", ModTime: modTime, Data: []byte("")},
			{Name: "templates/with-partials", ModTime: modTime, Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", ModTime: modTime, Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
		}
		opts.Templates = append(opts.Templates, sampleTemplates...)
	}
}

func withSampleSecret() chartOption {
	return func(opts *chartOptions) {
		sampleSecret := &common.File{Name: "templates/secret.yaml", ModTime: time.Now(), Data: []byte("apiVersion: v1\nkind: Secret\n")}
		opts.Templates = append(opts.Templates, sampleSecret)
	}
}

func withSampleIncludingIncorrectTemplates() chartOption {
	return func(opts *chartOptions) {
		modTime := time.Now()
		sampleTemplates := []*common.File{
			// This adds basic templates and partials.
			{Name: "templates/goodbye", ModTime: modTime, Data: []byte("goodbye: world")},
			{Name: "templates/empty", ModTime: modTime, Data: []byte("")},
			{Name: "templates/incorrect", ModTime: modTime, Data: []byte("{{ .Values.bad.doh }}")},
			{Name: "templates/with-partials", ModTime: modTime, Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", ModTime: modTime, Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
		}
		opts.Templates = append(opts.Templates, sampleTemplates...)
	}
}

func withMultipleManifestTemplate() chartOption {
	return func(opts *chartOptions) {
		sampleTemplates := []*common.File{
			{Name: "templates/rbac", ModTime: time.Now(), Data: []byte(rbacManifests)},
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
		Config:  map[string]any{"name": "value"},
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
		expectedDriverType any
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
			cfg := NewConfiguration()

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
		t.Error("Expected supported versions to at least include v1.")
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
				"templates/configmap.yaml": `
apiVersion: v1
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
				"templates/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value`,
				"templates/secret.yaml": `
apiVersion: v1
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
				"templates/multi.yaml": `
apiVersion: v1
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
				"templates/cm.yaml": `
apiVersion: v1
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
				"templates/empty.yaml": `
`,
			},
			expected: ``,
		},
		{
			name: "invalid yaml",
			files: map[string]string{
				"templates/invalid.yaml": `
invalid: yaml: content:
  - malformed`,
			},
			expectedError: "parsing templates/invalid.yaml",
		},
		{
			name: "leading doc separator glued to content by template whitespace trimming",
			files: map[string]string{
				"templates/service.yaml": `
---apiVersion: v1
kind: Service
metadata:
  name: test-svc
`,
			},
			expected: `apiVersion: v1
kind: Service
metadata:
  name: test-svc
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/service.yaml'
`,
		},
		{
			name: "leading doc separator on its own line",
			files: map[string]string{
				"templates/service.yaml": `
---
apiVersion: v1
kind: Service
metadata:
  name: test-svc
`,
			},
			expected: `apiVersion: v1
kind: Service
metadata:
  name: test-svc
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/service.yaml'
`,
		},
		{
			name: "multiple leading doc separators",
			files: map[string]string{
				"templates/service.yaml": `
---
---
apiVersion: v1
kind: Service
metadata:
  name: test-svc
`,
			},
			expected: `apiVersion: v1
kind: Service
metadata:
  name: test-svc
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/service.yaml'
`,
		},
		{
			name: "mid-content doc separator glued to content by template whitespace trimming",
			files: map[string]string{
				"templates/all.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
---apiVersion: v1
kind: Service
metadata:
  name: test-svc
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/all.yaml'
---
apiVersion: v1
kind: Service
metadata:
  name: test-svc
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/all.yaml'
`,
		},
		{
			name: "ConfigMap with embedded CA certificate",
			files: map[string]string{
				"templates/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ca-bundle
data:
  ca.crt: |
    ------BEGIN CERTIFICATE------
    MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zAKBggqhkjOPQQDAzASMRAw
    DgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYwMDAw
    WjASMRAwDgYDVQQKEwdBY21lIENvMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7Rmm
    ------END CERTIFICATE------
    ------BEGIN CERTIFICATE------
    MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zAKBggqhkjOPQQDAzASMRAw
    DgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYwMDAw
    WjASMRAwDgYDVQQKEwdBY21lIENvMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7Rmm
    ------END CERTIFICATE------
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: ca-bundle
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/configmap.yaml'
data:
  ca.crt: |
    ------BEGIN CERTIFICATE------
    MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zAKBggqhkjOPQQDAzASMRAw
    DgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYwMDAw
    WjASMRAwDgYDVQQKEwdBY21lIENvMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7Rmm
    ------END CERTIFICATE------
    ------BEGIN CERTIFICATE------
    MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zAKBggqhkjOPQQDAzASMRAw
    DgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYwMDAw
    WjASMRAwDgYDVQQKEwdBY21lIENvMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE7Rmm
    ------END CERTIFICATE------
`,
		},
		{
			name: "consecutive dashes in YAML value are not treated as document separators",
			files: map[string]string{
				"templates/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  config: |
    # ---------------------------------------------------------------------------
    [section]
    key = value
    # ---------------------------------------------------------------------------
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/configmap.yaml'
data:
  config: |
    # ---------------------------------------------------------------------------
    [section]
    key = value
    # ---------------------------------------------------------------------------
`,
		},
		{
			name: "JSON with dashes in values is not corrupted",
			files: map[string]string{
				"templates/dashboard.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard
data:
  dashboard.json: |
    {"options":{"---------":{"color":"#292929","text":"N/A"}}}
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/dashboard.yaml'
data:
  dashboard.json: |
    {"options":{"---------":{"color":"#292929","text":"N/A"}}}
`,
		},

		// **Note for Chart API v3**: This input should return an _ERROR_ in Chart API v3.
		// See the comment on the releaseutil.SplitManifests function for more details.
		{
			name: "multiple glued separators in same file",
			files: map[string]string{
				"templates/multi.yaml": `
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/multi.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/multi.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/multi.yaml'
`,
		},

		// **Note for Chart API v3**: This input should return an _ERROR_ in Chart API v3.
		// See the comment on the releaseutil.SplitManifests function for more details.
		{
			name: "mixed glued and proper separators",
			files: map[string]string{
				"templates/mixed.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
---apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/mixed.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/mixed.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/mixed.yaml'
`,
		},
		{
			name: "12 documents preserve in-file order",
			files: map[string]string{
				"templates/many.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-01
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-02
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-03
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-04
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-05
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-06
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-07
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-08
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-09
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-10
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-11
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-12
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-01
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-02
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-03
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-04
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-05
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-06
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-07
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-08
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-09
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-10
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-11
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-12
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/many.yaml'
`,
		},

		// Block scalar chomping indicator tests using | (clip), |- (strip), and |+ (keep)
		// inputs with 0, 1, and 2 trailing newlines after the block content.
		// Note: the emitter may normalize the output chomping indicator when the
		// trailing newline count makes another indicator equivalent for the result.

		// | (clip) input — clips trailing newlines to exactly one, though with
		// 0 trailing newlines the emitted output may normalize to |-.
		{
			name: "block scalar clip (|) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
`,
		},
		{
			name: "block scalar clip (|) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |
    hello
`,
		},
		{
			name: "block scalar clip (|) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello

`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |
    hello
`,
		},

		// |- (strip) — strips all trailing newlines
		{
			name: "block scalar strip (|-) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
`,
		},
		{
			name: "block scalar strip (|-) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
`,
		},
		{
			name: "block scalar strip (|-) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello

`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
`,
		},

		// |+ (keep) — preserves all trailing newlines
		{
			name: "block scalar keep (|+) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
`,
		},
		{
			name: "block scalar keep (|+) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello
`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |
    hello
`,
		},
		{
			name: "block scalar keep (|+) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello

`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |+
    hello

`,
		},

		// Multi-doc tests: block scalar doc is NOT the last document.
		// SplitManifests' regex consumes \s*\n before ---, so trailing
		// newlines from non-last docs are always stripped.

		// | (clip) in multi-doc (first doc)
		{
			name: "multi-doc block scalar clip (|) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar clip (|) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar clip (|) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |
    hello


---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},

		// |- (strip) in multi-doc (first doc)
		{
			name: "multi-doc block scalar strip (|-) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar strip (|-) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar strip (|-) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |-
    hello


---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},

		// |+ (keep) in multi-doc (first doc)
		{
			name: "multi-doc block scalar keep (|+) with 0 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar keep (|+) with 1 trailing newline",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
		},
		{
			name: "multi-doc block scalar keep (|+) with 2 trailing newlines",
			files: map[string]string{
				"templates/cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: |+
    hello


---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
data:
  val: simple`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  key: |-
    hello
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
  annotations:
    postrenderer.helm.sh/postrender-filename: 'templates/cm.yaml'
data:
  val: simple
`,
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
				"generated-by-postrender-test-0.yaml": `apiVersion: v1
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
			files, err := splitAndDeannotate(tt.input, "test")

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, files, len(tt.expectedFiles))

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
	reconstructed, err := splitAndDeannotate(merged, "test")
	require.NoError(t, err)

	// Compare the results
	assert.Len(t, reconstructed, len(originalFiles))
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
	values := map[string]any{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Empty(t, notes)
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
  name: value
`

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
	values := map[string]any{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
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
			{Name: "templates/invalid", ModTime: time.Now(), Data: []byte("invalid: yaml: content:")},
		},
	}
	values := map[string]any{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
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
	values := map[string]any{}

	_, _, _, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
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
	values := map[string]any{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Empty(t, notes) // Notes should be empty for this test

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
	values := map[string]any{}

	hooks, buf, notes, err := cfg.renderResources(
		ch, values, "test-release", "", false, false, false,
		nil, false, false, false, PostRenderStrategyCombined,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hooks)
	assert.NotNil(t, buf)
	assert.Empty(t, notes)
}

func TestRenderResources_PostRenderer_DuplicateResourceInHookAndTemplate(t *testing.T) {
	cfg := actionConfigFixture(t)

	// Simulate a chart where the same ServiceAccount appears both as a
	// pre-install hook and as a regular template. This is a valid Helm pattern
	// but previously caused post-renderers like Kustomize to fail with
	// "may not add resource with an already registered id" because hooks and
	// templates were merged into a single stream before post-rendering.
	saHook := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  annotations:
    "helm.sh/hook": pre-install
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded`

	saTemplate := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app`

	deployment := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      serviceAccountName: my-app`

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/sa-hook.yaml", ModTime: modTime, Data: []byte(saHook)},
		{Name: "templates/sa.yaml", ModTime: modTime, Data: []byte(saTemplate)},
		{Name: "templates/deployment.yaml", ModTime: modTime, Data: []byte(deployment)},
	})

	// Use a post-renderer that rejects duplicate resource IDs, similar to
	// how Kustomize behaves. We verify that no single post-render call
	// receives the ServiceAccount twice.
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			count := strings.Count(content, "kind: ServiceAccount")
			if count > 1 {
				t.Errorf("post-renderer received %d ServiceAccount resources in a single stream, expected at most 1", count)
			}
			return content
		},
	}

	hooks, buf, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategySeparate,
	)

	assert.NoError(t, err)
	assert.Len(t, hooks, 1)
	assert.Equal(t, "my-app", hooks[0].Name)
	assert.Contains(t, buf.String(), "kind: Deployment")
	assert.Contains(t, buf.String(), "kind: ServiceAccount")
}

func TestRenderResources_PostRenderer_CombinedInvokesOnceWithEverything(t *testing.T) {
	cfg := actionConfigFixture(t)

	hookManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-cm
  annotations:
    "helm.sh/hook": pre-install`
	templateManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/hook.yaml", ModTime: modTime, Data: []byte(hookManifest)},
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(templateManifest)},
	})

	var calls int
	var lastInput string
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			calls++
			lastInput = content
			return content
		},
	}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyCombined,
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "combined strategy should invoke the post-renderer exactly once")
	assert.Contains(t, lastInput, "hook-cm")
	assert.Contains(t, lastInput, "template-cm")
}

func TestRenderResources_PostRenderer_ZeroValueStrategyActsAsCombined(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`)},
		{Name: "templates/hook.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-cm
  annotations:
    "helm.sh/hook": pre-install`)},
	})

	var calls int
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			calls++
			return content
		},
	}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategy(""),
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "unset strategy must preserve backwards-compatible combined behavior")
}

func TestRenderResources_PostRenderer_SeparateSplitsHooksAndTemplates(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/hook.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-cm
  annotations:
    "helm.sh/hook": pre-install`)},
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`)},
	})

	var inputs []string
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			inputs = append(inputs, content)
			return content
		},
	}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategySeparate,
	)

	assert.NoError(t, err)
	assert.Len(t, inputs, 2, "separate strategy should invoke the post-renderer twice when both hooks and templates exist")
	for _, in := range inputs {
		hasHook := strings.Contains(in, "hook-cm")
		hasTemplate := strings.Contains(in, "template-cm")
		assert.False(t, hasHook && hasTemplate, "a single post-render invocation must not contain both hook and template resources")
		assert.True(t, hasHook || hasTemplate, "each post-render invocation must contain either a hook or a template")
	}
}

func TestRenderResources_PostRenderer_SeparateWithOnlyTemplates(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`)},
	})

	var calls int
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			calls++
			return content
		},
	}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategySeparate,
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "separate strategy should skip the empty hook group and invoke the post-renderer only once")
}

func TestRenderResources_PostRenderer_NoHooksSkipsHooks(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/hook.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-cm
  annotations:
    "helm.sh/hook": pre-install`)},
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`)},
	})

	var inputs []string
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			inputs = append(inputs, content)
			return content
		},
	}

	hooks, manifestDoc, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyNoHooks,
	)

	assert.NoError(t, err)
	assert.Len(t, inputs, 1, "nohooks strategy should invoke the post-renderer exactly once (for templates only)")
	assert.NotContains(t, inputs[0], "hook-cm", "hooks must not be sent to the post-renderer")
	assert.Contains(t, inputs[0], "template-cm", "templates must be sent to the post-renderer")

	// Hooks still round-trip through the release so they can execute.
	require.Len(t, hooks, 1)
	assert.Contains(t, hooks[0].Manifest, "hook-cm")
	assert.Contains(t, manifestDoc.String(), "template-cm")
}

func TestRenderResources_PostRenderer_NoHooksWithOnlyHooks(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/hook.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-cm
  annotations:
    "helm.sh/hook": pre-install`)},
	})

	var calls int
	mockPR := &mockPostRenderer{
		transform: func(content string) string {
			calls++
			return content
		},
	}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategyNoHooks,
	)

	assert.NoError(t, err)
	assert.Equal(t, 0, calls, "nohooks strategy should not invoke the post-renderer when the chart only has hooks")
}

func TestRenderResources_PostRenderer_UnknownStrategyErrors(t *testing.T) {
	cfg := actionConfigFixture(t)

	modTime := time.Now()
	ch := buildChartWithTemplates([]*common.File{
		{Name: "templates/cm.yaml", ModTime: modTime, Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: template-cm`)},
	})

	mockPR := &mockPostRenderer{}

	_, _, _, err := cfg.renderResources(
		ch, nil, "test-release", "", false, false, false,
		mockPR, false, false, false, PostRenderStrategy("bogus"),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown post-render strategy")
	assert.Contains(t, err.Error(), "bogus")
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
