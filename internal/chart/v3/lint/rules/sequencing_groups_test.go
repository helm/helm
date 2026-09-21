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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/internal/chart/v3/lint/support"
	release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/internal/release/v2/resourcegroup"
)

func TestSequencing_ResourceGroupDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		templates     map[string]string
		wantSeverity  int
		wantSubstring string
	}{
		{
			name: "cycle",
			templates: map[string]string{
				"templates/a.yaml": groupLintManifestYAML("ConfigMap", "a", map[string]string{
					resourcegroup.AnnotationResourceGroup:           "a",
					resourcegroup.AnnotationDependsOnResourceGroups: `["b"]`,
				}),
				"templates/b.yaml": groupLintManifestYAML("ConfigMap", "b", map[string]string{
					resourcegroup.AnnotationResourceGroup:           "b",
					resourcegroup.AnnotationDependsOnResourceGroups: `["a"]`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "resource-group circular dependency detected",
		},
		{
			name: "unknown group reference",
			templates: map[string]string{
				"templates/app.yaml": groupLintManifestYAML("ConfigMap", "app", map[string]string{
					resourcegroup.AnnotationResourceGroup:           "app",
					resourcegroup.AnnotationDependsOnResourceGroups: `["database"]`,
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: `depends-on non-existent group "database"`,
		},
		{
			name: "malformed dependency JSON",
			templates: map[string]string{
				"templates/app.yaml": groupLintManifestYAML("ConfigMap", "app", map[string]string{
					resourcegroup.AnnotationResourceGroup:           "app",
					resourcegroup.AnnotationDependsOnResourceGroups: "not-json",
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "invalid JSON",
		},
		{
			name: "duplicate resource membership",
			templates: map[string]string{
				"templates/first.yaml": groupLintManifestYAML("ConfigMap", "shared", map[string]string{
					resourcegroup.AnnotationResourceGroup: "first",
				}),
				"templates/second.yaml": groupLintManifestYAML("ConfigMap", "shared", map[string]string{
					resourcegroup.AnnotationResourceGroup: "second",
				}),
			},
			wantSeverity:  support.ErrorSev,
			wantSubstring: "assigned to multiple resource groups",
		},
		{
			name: "isolated groups",
			templates: map[string]string{
				"templates/a.yaml": groupLintManifestYAML("ConfigMap", "a", map[string]string{
					resourcegroup.AnnotationResourceGroup: "a",
				}),
				"templates/b.yaml": groupLintManifestYAML("ConfigMap", "b", map[string]string{
					resourcegroup.AnnotationResourceGroup: "b",
				}),
			},
			wantSeverity:  support.WarningSev,
			wantSubstring: "isolated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			messages := runSequencingLint(t, newSequencingChart("testchart", tt.templates))
			requireLintMessage(t, messages, tt.wantSeverity, tt.wantSubstring)
		})
	}
}

func TestSequencing_CleanResourceGroups(t *testing.T) {
	t.Parallel()

	templates := map[string]string{
		"templates/database.yaml": groupLintManifestYAML("ConfigMap", "database", map[string]string{
			resourcegroup.AnnotationResourceGroup: "database",
		}),
		"templates/app.yaml": groupLintManifestYAML("ConfigMap", "app", map[string]string{
			resourcegroup.AnnotationResourceGroup:           "app",
			resourcegroup.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}

	assert.Empty(t, runSequencingLint(t, newSequencingChart("testchart", templates)))
}

func TestSequencing_HookResourcesExcludedFromResourceGroups(t *testing.T) {
	t.Parallel()

	templates := map[string]string{
		"templates/base.yaml": groupLintManifestYAML("ConfigMap", "base", map[string]string{
			resourcegroup.AnnotationResourceGroup: "base",
		}),
		"templates/app.yaml": groupLintManifestYAML("ConfigMap", "app", map[string]string{
			resourcegroup.AnnotationResourceGroup:           "app",
			resourcegroup.AnnotationDependsOnResourceGroups: `["base"]`,
		}),
		"templates/hook.yaml": groupLintManifestYAML("Job", "hook", map[string]string{
			release.HookAnnotation:                          "pre-install",
			resourcegroup.AnnotationResourceGroup:           "base",
			resourcegroup.AnnotationDependsOnResourceGroups: `["app"]`,
		}),
	}

	assert.Empty(t, runSequencingLint(t, newSequencingChart("testchart", templates)))
}

func groupLintManifestYAML(kind, name string, annotations map[string]string) string {
	var manifest strings.Builder
	fmt.Fprintf(&manifest, "apiVersion: v1\nkind: %s\nmetadata:\n  name: %s\n", kind, name)
	if len(annotations) == 0 {
		return manifest.String()
	}

	manifest.WriteString("  annotations:\n")
	keys := make([]string, 0, len(annotations))
	for key := range annotations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&manifest, "    %s: %q\n", key, annotations[key])
	}
	return manifest.String()
}
