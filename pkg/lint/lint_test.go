package lint

import (
	"strings"

	"testing"
)

const badChartDir = "testdata/badchartversion"
const badYamlFileDir = "testdata/albatross"
const goodChartDir = "testdata/goodone"

func TestBadChart(t *testing.T) {
	m := All(badChartDir)
	if len(m) != 3 {
		t.Errorf("All didn't fail with expected errors, got %#v", m)
	}
	// There should be INFO, WARNING and ERROR messages, check for them
	var i, w, e = false, false, false
	for _, msg := range m {
		if msg.Severity == InfoSev {
			if strings.Contains(msg.Text, "values.toml") {
				i = true
			}
		}
		if msg.Severity == WarningSev {
			if strings.Contains(msg.Text, "No templates") {
				w = true
			}
		}
		if msg.Severity == ErrorSev {
			if strings.Contains(msg.Text, "Chart.yaml does not exist") {
				e = true
			}
		}
	}
	if !i || !w || !e {
		t.Errorf("Didn't find all the expected errors, got %#v", m)
	}
}

func TestInvalidYaml(t *testing.T) {
	m := All(badYamlFileDir)
	if len(m) != 1 {
		t.Errorf("All didn't fail with expected errors")
	}
	if !strings.Contains(m[0].Text, "deliberateSyntaxError") {
		t.Errorf("All didn't have the error for deliberateSyntaxError")
	}
}

func TestGoodChart(t *testing.T) {
	m := All(goodChartDir)
	if len(m) != 0 {
		t.Errorf("All failed but shouldn't have: %#v", m)
	}
}
