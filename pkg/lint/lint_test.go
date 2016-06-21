package lint

import (
	"k8s.io/helm/pkg/lint/support"
	"strings"

	"testing"
)

const badChartDir = "rules/testdata/badchartfile"
const badValuesFileDir = "rules/testdata/badvaluesfile"
const badYamlFileDir = "rules/testdata/albatross"
const goodChartDir = "rules/testdata/goodone"

func TestBadChart(t *testing.T) {
	m := All(badChartDir)
	if len(m) != 4 {
		t.Errorf("Number of errors %v", len(m))
		t.Errorf("All didn't fail with expected errors, got %#v", m)
	}
	// There should be 2 WARNINGs and one ERROR messages, check for them
	var w, e, e2, e3 bool
	for _, msg := range m {
		if msg.Severity == support.WarningSev {
			if strings.Contains(msg.Text, "Templates directory not found") {
				w = true
			}
		}
		if msg.Severity == support.ErrorSev {
			if strings.Contains(msg.Text, "'version' 0.0.0 is less than or equal to 0") {
				e = true
			}
			if strings.Contains(msg.Text, "'name' is required") {
				e2 = true
			}
			if strings.Contains(msg.Text, "'name' and directory do not match") {
				e3 = true
			}
		}
	}
	if !e || !e2 || !e3 || !w {
		t.Errorf("Didn't find all the expected errors, got %#v", m)
	}
}

func TestInvalidYaml(t *testing.T) {
	m := All(badYamlFileDir)
	if len(m) != 1 {
		t.Errorf("All didn't fail with expected errors, got %#v", m)
	}
	if !strings.Contains(m[0].Text, "deliberateSyntaxError") {
		t.Errorf("All didn't have the error for deliberateSyntaxError")
	}
}

func TestBadValues(t *testing.T) {
	m := All(badValuesFileDir)
	if len(m) != 1 {
		t.Errorf("All didn't fail with expected errors, got %#v", m)
	}
	if !strings.Contains(m[0].Text, "cannot unmarshal") {
		t.Errorf("All didn't have the error for invalid key format: %s", m[0].Text)
	}
}

func TestGoodChart(t *testing.T) {
	m := All(goodChartDir)
	if len(m) != 0 {
		t.Errorf("All failed but shouldn't have: %#v", m)
	}
}
