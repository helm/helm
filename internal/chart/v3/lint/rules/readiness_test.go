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

package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/kube"
)

func TestReadinessRule_PresencePairs(t *testing.T) {
	t.Parallel()

	validSuccess := `["{.status.phase} == \"Ready\""]`
	validFailure := `["{.status.phase} == \"Failed\""]`

	tests := []struct {
		name          string
		annotations   map[string]string
		wantError     bool
		wantSubstring string
	}{
		{
			name: "success only",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: validSuccess,
			},
			wantError:     true,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "failure only",
			annotations: map[string]string{
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "blank is absent",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: "",
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "empty array is absent",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `[]`,
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "null is absent",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `null`,
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "both must be present or absent together",
		},
		{
			name: "malformed JSON",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `["{.status.phase} == \"Ready\""`,
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "malformed",
		},
		{
			name: "blank array element",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `[""]`,
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "expression cannot be empty",
		},
		{
			name: "non-numeric ordering literal",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `["{.status.phase} > \"Running\""]`,
				kube.AnnotationReadinessFailure: validFailure,
			},
			wantError:     true,
			wantSubstring: "requires a numeric comparison value",
		},
		{
			name:        "both absent",
			annotations: nil,
		},
		{
			name: "both valid",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: validSuccess,
				kube.AnnotationReadinessFailure: validFailure,
			},
		},
		{
			name: "quoted numeric ordering",
			annotations: map[string]string{
				kube.AnnotationReadinessSuccess: `["{.status.readyReplicas} >= \"1\""]`,
				kube.AnnotationReadinessFailure: `["{.status.failed} > \"0\""]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			messages := runReadinessLint(t, tt.annotations)
			if !tt.wantError {
				assert.Empty(t, messages)
				return
			}

			requireReadinessMessage(t, messages, support.ErrorSev, tt.wantSubstring)
		})
	}
}

func TestReadinessRule_HooksSkipped(t *testing.T) {
	t.Parallel()

	messages := runReadinessLint(t, map[string]string{
		release.HookAnnotation:          "pre-install",
		kube.AnnotationReadinessSuccess: `["{.status.phase} == \"Ready\""]`,
	})

	assert.Empty(t, messages)
}

func TestReadinessRule_QuietWithoutAnnotations(t *testing.T) {
	t.Parallel()

	assert.Empty(t, runReadinessLint(t, nil))
}

func runReadinessLint(t *testing.T, annotations map[string]string) []support.Message {
	t.Helper()

	chartDir := t.TempDir()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "readiness-test",
			Version:    "0.1.0",
			APIVersion: chart.APIVersionV3,
		},
		Templates: []*common.File{
			{
				Name: "templates/configmap.yaml",
				Data: []byte(readinessManifestYAML(annotations)),
			},
		},
	}
	require.NoError(t, chartutil.SaveDir(c, chartDir))

	linter := support.Linter{ChartDir: filepath.Join(chartDir, c.Name())}
	Readiness(&linter, nil, "test-namespace")
	return linter.Messages
}

func readinessManifestYAML(annotations map[string]string) string {
	var annotationBlock strings.Builder
	if len(annotations) > 0 {
		keys := make([]string, 0, len(annotations))
		for key := range annotations {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		annotationBlock.WriteString("  annotations:\n")
		for _, key := range keys {
			fmt.Fprintf(&annotationBlock, "    %s: %q\n", key, annotations[key])
		}
	}

	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: readiness-test
%sdata:
  key: value
`, annotationBlock.String())
}

func requireReadinessMessage(t *testing.T, messages []support.Message, severity int, substring string) {
	t.Helper()

	for _, message := range messages {
		if message.Severity == severity && strings.Contains(message.Err.Error(), substring) {
			return
		}
	}

	t.Fatalf("expected severity %d message containing %q, got %#v", severity, substring, messages)
}
