package rules

import (
	"k8s.io/helm/pkg/lint/support"
	"strings"
	"testing"
)

const templateTestBasedir = "./testdata/albatross"

func TestTemplate(t *testing.T) {
	linter := support.Linter{ChartDir: templateTestBasedir}
	Templates(&linter)
	res := linter.Messages

	if len(res) != 1 {
		t.Fatalf("Expected one error, got %d, %v", len(res), res)
	}

	if !strings.Contains(res[0].Text, "deliberateSyntaxError") {
		t.Errorf("Unexpected error: %s", res[0])
	}
}
