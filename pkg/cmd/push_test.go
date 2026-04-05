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

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestPushFileCompletion(t *testing.T) {
	checkFileCompletion(t, "push", true)
	checkFileCompletion(t, "push package.tgz", false)
	checkFileCompletion(t, "push package.tgz oci://localhost:5000", false)
}

func TestPushWriterTable(t *testing.T) {
	// WriteTable is intentionally a no-op: the registry client already prints
	// push details to stderr, so we avoid duplicating that output.
	w := &pushWriter{result: pushResult{
		Ref:    "oci://example.com/charts/mychart:1.0.0",
		Digest: "sha256:abc123",
	}}
	var buf bytes.Buffer
	if err := w.WriteTable(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("table output should be empty (registry client writes to stderr), got: %q", got)
	}
}

func TestPushWriterJSON(t *testing.T) {
	w := &pushWriter{result: pushResult{
		Ref:    "oci://example.com/charts/mychart:1.0.0",
		Digest: "sha256:abc123",
	}}
	var buf bytes.Buffer
	if err := w.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, `"ref"`) || !strings.Contains(got, `"digest"`) {
		t.Errorf("JSON output missing fields, got: %q", got)
	}
	if !strings.Contains(got, "oci://example.com/charts/mychart:1.0.0") {
		t.Errorf("JSON output missing Ref value, got: %q", got)
	}
	if !strings.Contains(got, "sha256:abc123") {
		t.Errorf("JSON output missing Digest value, got: %q", got)
	}
}

func TestPushWriterYAML(t *testing.T) {
	w := &pushWriter{result: pushResult{
		Ref:    "oci://example.com/charts/mychart:1.0.0",
		Digest: "sha256:abc123",
	}}
	var buf bytes.Buffer
	if err := w.WriteYAML(&buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "ref:") || !strings.Contains(got, "digest:") {
		t.Errorf("YAML output missing fields, got: %q", got)
	}
	if !strings.Contains(got, "oci://example.com/charts/mychart:1.0.0") {
		t.Errorf("YAML output missing Ref value, got: %q", got)
	}
	if !strings.Contains(got, "sha256:abc123") {
		t.Errorf("YAML output missing Digest value, got: %q", got)
	}
}
