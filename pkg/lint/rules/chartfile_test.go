package rules

import (
	"k8s.io/helm/pkg/lint/support"
	"strings"
	"testing"
)

const badchartfile = "testdata/badchartfile"

func TestChartfile(t *testing.T) {
	linter := support.Linter{ChartDir: badchartfile}
	Chartfile(&linter)
	msgs := linter.Messages

	if len(msgs) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(msgs))
	}

	if !strings.Contains(msgs[0].Text, "'name' is required") {
		t.Errorf("Unexpected message 0: %s", msgs[0].Text)
	}

	if !strings.Contains(msgs[1].Text, "'name' and directory do not match") {
		t.Errorf("Unexpected message 1: %s", msgs[1].Text)
	}

	if !strings.Contains(msgs[2].Text, "'version' 0.0.0 is less than or equal to 0") {
		t.Errorf("Unexpected message 2: %s", msgs[2].Text)
	}
}
