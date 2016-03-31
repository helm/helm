package pkg

import (
	"testing"
	"text/template"
)

// TestGoFeatures is a canary test to make sure that features that are invisible at API level are supported.
func TestGoFeatures(t *testing.T) {
	// Test that template with Go 1.6 syntax can compile.
	_, err := template.New("test").Parse(`{{- printf "hello"}}`)
	if err != nil {
		t.Fatalf("You must use a version of Go that supports {{- template syntax. (1.6+)")
	}
}
