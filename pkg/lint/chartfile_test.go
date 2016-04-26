package lint

import (
	"testing"
)

const badchartfile = "testdata/badchartfile"

func TestChartfile(t *testing.T) {
	msgs := Chartfile(badchartfile)
	if len(msgs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(msgs))
	}

	if msgs[0].Text != "Chart.yaml: 'name' is required" {
		t.Errorf("Unexpected message 0: %s", msgs[0].Text)
	}

	if msgs[1].Text != "Chart.yaml: 'version' is required, and must be greater than 0.0.0" {
		t.Errorf("Unexpected message 1: %s", msgs[1].Text)
	}
}
