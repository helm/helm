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

package lint

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

var values map[string]interface{}

const namespace = "testNamespace"
const strict = false
const defaultEnableFullPathFlag = false

const badChartDir = "rules/testdata/badchartfile"
const badValuesFileDir = "rules/testdata/badvaluesfile"
const badYamlFileDir = "rules/testdata/albatross"
const goodChartDir = "rules/testdata/goodone"
const subChartValuesDir = "rules/testdata/withsubchart"
const malformedTemplate = "rules/testdata/malformed-template"

func TestBadChart(t *testing.T) {
	badChartDirFullPath, err := filepath.Abs(badChartDir)
	if err != nil {
		t.Fatalf("Failed to determine the full path of templates directory")
	}

	t.Run("bad chart", func(t *testing.T) {
		linterMessages := All(badChartDir, values, namespace, strict, defaultEnableFullPathFlag).Messages
		if len(linterMessages) != 8 {
			t.Errorf("Number of errors %v", len(linterMessages))
			t.Errorf("All didn't fail with expected errors, got %#v", linterMessages)
		}

		type testCase struct {
			name               string
			severity           int
			matcher            func(err error) bool
			expectedMatchCount int
		}

		tests := []testCase{
			{
				name:     "info: icon is recommended",
				severity: support.InfoSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "icon is recommended")
				},
				expectedMatchCount: 1,
			},
			{
				name:     "error: version '0.0.0.0' is not a valid SemVer",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "version '0.0.0.0' is not a valid SemVer")
				},
				expectedMatchCount: 1,
			},
			{
				name:     "error: validation: chart.metadata.name is required and not unable to load chart",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "validation: chart.metadata.name is required") &&
						!strings.Contains(e.Error(), "unable to load chart")
				},
				expectedMatchCount: 1,
			},
			{
				name:     `error: apiVersion is required. The value must be either "v1" or "v2"`,
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), `apiVersion is required. The value must be either "v1" or "v2"`)
				},
				expectedMatchCount: 1,
			},
			{
				name:     "error: chart type is not valid in apiVersion",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "chart type is not valid in apiVersion")
				},
				expectedMatchCount: 1,
			},
			{
				name:     "error: dependencies are not valid in the Chart file with apiVersion",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "dependencies are not valid in the Chart file with apiVersion")
				},
				expectedMatchCount: 1,
			},
			{
				name:     "error: unable to load chart",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "unable to load chart")
				},
				expectedMatchCount: 1,
				// This comes from the dependency check, which loads dependency info
				// from the Chart.yaml file. Path will be empty for this.
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				matchCount := 0

				for _, msg := range linterMessages {
					if msg.Severity != tt.severity {
						// If severity of the message doesn't match, ignore the matcher
						continue
					}

					if !tt.matcher(msg.Err) {
						// If error matcher func doesn't return true, ignore counting current message
						continue
					}
					matchCount++

					// Message shouldn't contain full path unless the flag --full-path is enabled
					if strings.HasPrefix(msg.Path, badChartDirFullPath) {
						t.Errorf("Full path is not expected when --full-path is disabled: %s",
							msg.Path)
					}
				}

				// Expected exact match count. Else, update matcher-func or count.
				if matchCount < tt.expectedMatchCount {
					t.Errorf("Didn't find all the expected errors")
				} else if matchCount > tt.expectedMatchCount {
					t.Errorf("Too many matches found for the current error-matcher-func: %d", matchCount)
				}
			})
		}
	})

	t.Run("bad chart --full-path enabled", func(t *testing.T) {
		linterMessages := All(badChartDir, values, namespace, strict, true).Messages
		if len(linterMessages) != 8 {
			t.Errorf("Number of errors %v", len(linterMessages))
			t.Errorf("All didn't fail with expected errors, got %#v", linterMessages)
		}

		type testCase struct {
			name               string
			severity           int
			matcher            func(err error) bool
			expectedMatchCount int
			checkFullPath      bool
		}

		tests := []testCase{
			{
				name:     "info: icon is recommended",
				severity: support.InfoSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "icon is recommended")
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     "error: version '0.0.0.0' is not a valid SemVer",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "version '0.0.0.0' is not a valid SemVer")
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     "error: validation: chart.metadata.name is required and not unable to load chart",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "validation: chart.metadata.name is required") &&
						!strings.Contains(e.Error(), "unable to load chart")
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     `error: apiVersion is required. The value must be either "v1" or "v2"`,
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), `apiVersion is required. The value must be either "v1" or "v2"`)
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     "error: chart type is not valid in apiVersion",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "chart type is not valid in apiVersion")
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     "error: dependencies are not valid in the Chart file with apiVersion",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "dependencies are not valid in the Chart file with apiVersion")
				},
				expectedMatchCount: 1,
				checkFullPath:      true,
			},
			{
				name:     "error: unable to load chart",
				severity: support.ErrorSev,
				matcher: func(e error) bool {
					return strings.Contains(e.Error(), "unable to load chart")
				},
				expectedMatchCount: 1,
				// This comes from the dependency check, which loads dependency info
				// from the Chart.yaml file. Path will be empty for this.
				checkFullPath: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				matchCount := 0

				for _, msg := range linterMessages {
					if msg.Severity != tt.severity {
						// If severity of the message doesn't match, ignore the matcher
						continue
					}

					if !tt.matcher(msg.Err) {
						// If error matcher func doesn't return true, ignore counting current message
						continue
					}
					matchCount++

					if !tt.checkFullPath {
						// When check for full path is disable for given matcher
						continue
					}

					if !strings.HasPrefix(msg.Path, badChartDirFullPath) {
						// If the message doesn't have chart directory's full path in prefix, fail the sub-test
						t.Errorf("Absolute path missing or incorrect: %s: %v\n"+
							"Should contain prefix: %s", msg.Path, err, badChartDirFullPath)
					}
				}

				// Expected exact match count. Else, update matcher-func or count.
				if matchCount < tt.expectedMatchCount {
					t.Errorf("Didn't find all the expected errors")
				} else if matchCount > tt.expectedMatchCount {
					t.Errorf("Too many matches found for the current error-matcher-func: %d", matchCount)
				}
			})
		}
	})
}

func TestInvalidYaml(t *testing.T) {
	chartPath := badYamlFileDir
	badYamlFileDirFullPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatalf("Failed to determine the full path of bad YAML file's directory")
	}

	templatesFileFullPath := filepath.Join(badYamlFileDirFullPath, "templates") +
		string(filepath.Separator)

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "invalid YAML",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "invalid YAML with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := All(chartPath, values, namespace, strict, tt.enableFullPath).Messages
			if len(m) != 1 {
				t.Fatalf("All didn't fail with expected errors, got %#v", m)
			}
			if !strings.Contains(m[0].Err.Error(), "deliberateSyntaxError") {
				t.Errorf("All didn't have the error for deliberateSyntaxError")
			}

			if tt.enableFullPath {
				if m[0].Path != templatesFileFullPath {
					t.Errorf("Full path is missing or incorrect\nExpected: %s\nGot     : %s",
						templatesFileFullPath, m[0].Path)
				}
			}
		})
	}
}

func TestBadValues(t *testing.T) {
	chartPath := badValuesFileDir
	valuesFileFullPath, err := filepath.Abs(filepath.Join(chartPath, "values.yaml"))
	if err != nil {
		t.Fatalf("Failed to determine the full path of bad values file")
	}

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "bad values",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "bad values with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := All(chartPath, values, namespace, strict, tt.enableFullPath).Messages
			if len(m) < 1 {
				t.Fatalf("All didn't fail with expected errors, got %#v", m)
			}

			if !strings.Contains(m[0].Err.Error(), "unable to parse YAML") {
				t.Errorf("All didn't have the error for invalid key format: %s", m[0].Err)
			}

			if tt.enableFullPath {
				if m[0].Path != valuesFileFullPath {
					t.Errorf("Full path is missing or incorrect\nExpected: %s\nGot     : %s",
						valuesFileFullPath, m[0].Path)
				}
			}
		})
	}
}

func TestGoodChart(t *testing.T) {
	chartPath := goodChartDir
	goodChartDirFullPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatalf("Failed to determine the full path of good chart")
	}

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "good chart",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "good chart with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := All(chartPath, values, namespace, strict, tt.enableFullPath).Messages
			if len(m) != 0 {
				t.Error("All returned linter messages when it shouldn't have")
				for i, msg := range m {
					t.Logf("Message %d: %s", i, msg)

					if tt.enableFullPath {
						// Check if the message has directory path prefixed, i.e., has full path.
						if !strings.HasPrefix(msg.Path, goodChartDirFullPath) {
							t.Errorf("Absolute path missing or incorrect: %s\n"+
								"Should contain prefix: %s", msg.Path, goodChartDirFullPath)
						}
					}
				}
			}
		})
	}
}

// TestHelmCreateChart tests that a `helm create` always passes a `helm lint` test.
//
// See https://github.com/helm/helm/issues/7923
func TestHelmCreateChart(t *testing.T) {
	dir := t.TempDir()

	createdChart, err := chartutil.Create("testhelmcreatepasseslint", dir)
	if err != nil {
		t.Error(err)
		// Fatal is bad because of the defer.
		return
	}

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "create chart and lint",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "create chart and lint with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: we test with strict=true here, even though others have
			// strict = false.
			m := All(createdChart, values, namespace, true, tt.enableFullPath).Messages
			if ll := len(m); ll != 1 {
				t.Errorf("All should have had exactly 1 error. Got %d", ll)
				for i, msg := range m {
					t.Logf("Message %d: %s", i, msg.Error())
				}
			} else if msg := m[0].Err.Error(); !strings.Contains(msg, "icon is recommended") {
				t.Errorf("Unexpected lint error: %s", msg)
			}

			if tt.enableFullPath {
				// Check if the message has directory path prefixed, i.e., has full path.
				// Note: t.TempDir() returns the absolute path to temporary directory.
				if !strings.HasPrefix(m[0].Path, dir) {
					t.Errorf("Absolute path missing or incorrect: %s\n"+
						"Should contain prefix: %s", m[0].Path, dir)
				}
			}
		})
	}
}

// lint ignores import-values
// See https://github.com/helm/helm/issues/9658
func TestSubChartValuesChart(t *testing.T) {
	chartPath := subChartValuesDir
	subChartValuesDirFullPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatalf("Failed to determine the full path of sub-chart's directory")
	}

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "sub-chart values",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "sub-chart values with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := All(chartPath, values, namespace, strict, tt.enableFullPath).Messages
			if len(m) != 0 {
				t.Error("All returned linter messages when it shouldn't have")
				for i, msg := range m {
					t.Logf("Message %d: %s", i, msg)

					if tt.enableFullPath {
						// Check if the message has directory path prefixed, i.e., has full path.
						if !strings.HasPrefix(msg.Path, subChartValuesDirFullPath) {
							t.Errorf("Absolute path missing or incorrect: %s\n"+
								"Should contain prefix: %s", msg.Path, subChartValuesDirFullPath)
						}
					}
				}
			}
		})
	}
}

// lint stuck with malformed template object
// See https://github.com/helm/helm/issues/11391
func TestMalformedTemplate(t *testing.T) {
	chartPath := malformedTemplate
	malformedTemplateFullPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatalf("Failed to determine the full path of malformed template's directory")
	}

	tests := []struct {
		name           string
		enableFullPath bool
	}{
		{
			name:           "malformed template",
			enableFullPath: defaultEnableFullPathFlag,
		},
		{
			name:           "malformed template with --full-path enabled",
			enableFullPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := time.After(3 * time.Second)
			ch := make(chan int, 1)
			var m []support.Message
			go func() {
				m = All(chartPath, values, namespace, strict, tt.enableFullPath).Messages
				ch <- 1
			}()
			select {
			case <-c:
				t.Fatalf("lint malformed template timeout")
			case <-ch:
				if len(m) != 1 {
					t.Fatalf("All didn't fail with expected errors, got %#v", m)
				}

				if !strings.Contains(m[0].Err.Error(), "invalid character '{'") {
					t.Errorf("All didn't have the error for invalid character '{'")
				}

				if tt.enableFullPath {
					// Check if the message has directory path prefixed, i.e., has full path.
					if !strings.HasPrefix(m[0].Path, malformedTemplateFullPath) {
						t.Errorf("Absolute path missing or incorrect: %s\n"+
							"Should contain prefix: %s", m[0].Path, malformedTemplateFullPath)
					}
				}
			}
		})
	}
}
