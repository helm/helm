package lint

import (
	"strings"
	"testing"
)

const templateTestBasedir = "./testdata/albatross"

func TestTemplate(t *testing.T) {
	res := Templates(templateTestBasedir)

	if len(res) != 1 {
		t.Fatalf("Expected one error, got %d", len(res))
	}

	if !strings.Contains(res[0].Text, "deliberateSyntaxError") {
		t.Errorf("Unexpected error: %s", res[0])
	}
}
