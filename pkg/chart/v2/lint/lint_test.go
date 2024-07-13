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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

const namespace = "testNamespace"

const badChartDir = "rules/testdata/badchartfile"
const badValuesFileDir = "rules/testdata/badvaluesfile"
const badYamlFileDir = "rules/testdata/albatross"
const badCrdFileDir = "rules/testdata/badcrdfile"
const goodChartDir = "rules/testdata/goodone"
const subChartValuesDir = "rules/testdata/withsubchart"
const malformedTemplate = "rules/testdata/malformed-template"
const invalidChartFileDir = "rules/testdata/invalidchartfile"

func TestBadChart(t *testing.T) {
	var values map[string]any
	m := RunAll(badChartDir, values, namespace).Messages
	if len(m) != 9 {
		t.Errorf("Number of errors %v", len(m))
		t.Errorf("All didn't fail with expected errors, got %#v", m)
	}
	// There should be one INFO, 2 WARNING and 2 ERROR messages, check for them
	var i, w, w2, e, e2, e3, e4, e5, e6 bool
	for _, msg := range m {
		if msg.Severity == support.InfoSev {
			if strings.Contains(msg.Err.Error(), "icon is recommended") {
				i = true
			}
		}
		if msg.Severity == support.WarningSev {
			if strings.Contains(msg.Err.Error(), "does not exist") {
				w = true
			}
		}
		if msg.Severity == support.ErrorSev {
			if strings.Contains(msg.Err.Error(), "version '0.0.0.0' is not a valid SemVer") {
				e = true
			}
			if strings.Contains(msg.Err.Error(), "name is required") {
				e2 = true
			}

			if strings.Contains(msg.Err.Error(), "apiVersion is required. The value must be either \"v1\" or \"v2\"") {
				e3 = true
			}

			if strings.Contains(msg.Err.Error(), "chart type is not valid in apiVersion") {
				e4 = true
			}

			if strings.Contains(msg.Err.Error(), "dependencies are not valid in the Chart file with apiVersion") {
				e5 = true
			}
			// This comes from the dependency check, which loads dependency info from the Chart.yaml
			if strings.Contains(msg.Err.Error(), "unable to load chart") {
				e6 = true
			}
		}
		if msg.Severity == support.WarningSev {
			if strings.Contains(msg.Err.Error(), "version '0.0.0.0' is not a valid SemVerV2") {
				w2 = true
			}
		}
	}
	if !e || !e2 || !e3 || !e4 || !e5 || !i || !e6 || !w || !w2 {
		t.Errorf("Didn't find all the expected errors, got %#v", m)
	}
}

func TestInvalidYaml(t *testing.T) {
	var values map[string]any
	m := RunAll(badYamlFileDir, values, namespace).Messages
	if len(m) != 1 {
		t.Fatalf("All didn't fail with expected errors, got %#v", m)
	}
	if !strings.Contains(m[0].Err.Error(), "deliberateSyntaxError") {
		t.Errorf("All didn't have the error for deliberateSyntaxError")
	}
}

func TestInvalidChartYaml(t *testing.T) {
	var values map[string]any
	m := RunAll(invalidChartFileDir, values, namespace).Messages
	if len(m) != 2 {
		t.Fatalf("All didn't fail with expected errors, got %#v", m)
	}
	if !strings.Contains(m[0].Err.Error(), "failed to strictly parse chart metadata file") {
		t.Errorf("All didn't have the error for duplicate YAML keys")
	}
}

func TestBadValues(t *testing.T) {
	var values map[string]any
	m := RunAll(badValuesFileDir, values, namespace).Messages
	if len(m) < 1 {
		t.Fatalf("All didn't fail with expected errors, got %#v", m)
	}
	if !strings.Contains(m[0].Err.Error(), "unable to parse YAML") {
		t.Errorf("All didn't have the error for invalid key format: %s", m[0].Err)
	}
}

func TestBadCrdFile(t *testing.T) {
	var values map[string]any
	m := RunAll(badCrdFileDir, values, namespace).Messages
	assert.Lenf(t, m, 2, "All didn't fail with expected errors, got %#v", m)
	assert.ErrorContains(t, m[0].Err, "apiVersion is not in 'apiextensions.k8s.io'")
	assert.ErrorContains(t, m[1].Err, "object kind is not 'CustomResourceDefinition'")
}

func TestGoodChart(t *testing.T) {
	var values map[string]any
	m := RunAll(goodChartDir, values, namespace).Messages
	if len(m) != 0 {
		t.Error("All returned linter messages when it shouldn't have")
		for i, msg := range m {
			t.Logf("Message %d: %s", i, msg)
		}
	}
}

// TestHelmCreateChart tests that a `helm create` always passes a `helm lint` test.
//
// See https://github.com/helm/helm/issues/7923
func TestHelmCreateChart(t *testing.T) {
	var values map[string]any
	dir := t.TempDir()

	createdChart, err := chartutil.Create("testhelmcreatepasseslint", dir)
	if err != nil {
		t.Error(err)
		// Fatal is bad because of the defer.
		return
	}

	// Note: we test with strict=true here, even though others have
	// strict = false.
	m := RunAll(createdChart, values, namespace, WithSkipSchemaValidation(true)).Messages
	if ll := len(m); ll != 1 {
		t.Errorf("All should have had exactly 1 error. Got %d", ll)
		for i, msg := range m {
			t.Logf("Message %d: %s", i, msg.Error())
		}
	} else if msg := m[0].Err.Error(); !strings.Contains(msg, "icon is recommended") {
		t.Errorf("Unexpected lint error: %s", msg)
	}
}

// TestHelmCreateChart_CheckDeprecatedWarnings checks if any default template created by `helm create` throws
// deprecated warnings in the linter check against the current Kubernetes version (provided using ldflags).
//
// See https://github.com/helm/helm/issues/11495
//
// Resources like hpa and ingress, which are disabled by default in values.yaml are enabled here using the equivalent
// of the `--set` flag.
func TestHelmCreateChart_CheckDeprecatedWarnings(t *testing.T) {
	createdChart, err := chartutil.Create("checkdeprecatedwarnings", t.TempDir())
	if err != nil {
		t.Error(err)
		return
	}

	// Add values to enable hpa, and ingress which are disabled by default.
	// This is the equivalent of:
	//   helm lint checkdeprecatedwarnings --set 'autoscaling.enabled=true,ingress.enabled=true'
	updatedValues := map[string]any{
		"autoscaling": map[string]any{
			"enabled": true,
		},
		"ingress": map[string]any{
			"enabled": true,
		},
	}

	linterRunDetails := RunAll(createdChart, updatedValues, namespace, WithSkipSchemaValidation(true))
	for _, msg := range linterRunDetails.Messages {
		if strings.HasPrefix(msg.Error(), "[WARNING]") &&
			strings.Contains(msg.Error(), "deprecated") {
			// When there is a deprecation warning for an object created
			// by `helm create` for the current Kubernetes version, fail.
			t.Errorf("Unexpected deprecation warning for %q: %s", msg.Path, msg.Error())
		}
	}
}

// lint ignores import-values
// See https://github.com/helm/helm/issues/9658
func TestSubChartValuesChart(t *testing.T) {
	var values map[string]any
	m := RunAll(subChartValuesDir, values, namespace).Messages
	if len(m) != 0 {
		t.Error("All returned linter messages when it shouldn't have")
		for i, msg := range m {
			t.Logf("Message %d: %s", i, msg)
		}
	}
}

// lint stuck with malformed template object
// See https://github.com/helm/helm/issues/11391
func TestMalformedTemplate(t *testing.T) {
	var values map[string]any
	c := time.After(3 * time.Second)
	ch := make(chan int, 1)
	var m []support.Message
	go func() {
		m = RunAll(malformedTemplate, values, namespace).Messages
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
	}
}
