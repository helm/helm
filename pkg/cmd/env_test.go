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
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestEnv(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "completion for env",
		cmd:    "__complete env ''",
		golden: "output/env-comp.txt",
	}, {
		name:   "completion for env output flag",
		cmd:    "__complete env --output ''",
		golden: "output/env-output-comp.txt",
	}, {
		name:      "env with invalid output format",
		cmd:       "env --output table",
		golden:    "output/env-output-invalid.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestEnvOutputDefault(t *testing.T) {
	// Pin the default format: without --output, every line must keep the
	// historic KEY="VALUE" shape. The values are machine-specific, so
	// assert the shape rather than compare against a golden file.
	_, out, err := executeActionCommand("env")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, `HELM_BIN="`) {
		t.Errorf("expected HELM_BIN in default output, got %q", out)
	}
	lineRe := regexp.MustCompile(`^[A-Z0-9_]+=".*"$`)
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if !lineRe.MatchString(line) {
			t.Errorf("line %q does not match KEY=\"VALUE\" format", line)
		}
	}

	// Single-variable mode without --output keeps printing the bare value.
	// Pin BurstLimit so the expected value cannot drift with the ambient
	// HELM_BURST_LIMIT of the test process.
	oldBurstLimit := settings.BurstLimit
	settings.BurstLimit = 150
	t.Cleanup(func() { settings.BurstLimit = oldBurstLimit })

	_, out, err = executeActionCommand("env HELM_BURST_LIMIT")
	if err != nil {
		t.Fatal(err)
	}
	if out != "150\n" {
		t.Errorf("expected %q, got %q", "150\n", out)
	}
}

func TestEnvOutputJSON(t *testing.T) {
	// The full listing contains machine-specific paths, so validate the
	// shape instead of comparing against a golden file.
	_, out, err := executeActionCommand("env --output json")
	if err != nil {
		t.Fatal(err)
	}

	var envVars map[string]string
	if err := json.Unmarshal([]byte(out), &envVars); err != nil {
		t.Fatalf("expected valid JSON output, got %q: %s", out, err)
	}
	if _, ok := envVars["HELM_BIN"]; !ok {
		t.Errorf("expected HELM_BIN in JSON output, got %v", envVars)
	}
}

func TestEnvFormatWrite(t *testing.T) {
	envVars := map[string]string{
		"HELM_BIN":       "/usr/local/bin/helm",
		"HELM_DEBUG":     "false",
		"HELM_NAMESPACE": "default",
	}

	tests := []struct {
		name   string
		format envFormat
		want   string
	}{{
		name:   "key=value format",
		format: envFormatKeyValue,
		want: `HELM_BIN="/usr/local/bin/helm"
HELM_DEBUG="false"
HELM_NAMESPACE="default"
`,
	}, {
		name:   "json format",
		format: envFormatJSON,
		want:   `{"HELM_BIN":"/usr/local/bin/helm","HELM_DEBUG":"false","HELM_NAMESPACE":"default"}` + "\n",
	}, {
		name:   "yaml format",
		format: envFormatYAML,
		want: `HELM_BIN: /usr/local/bin/helm
HELM_DEBUG: "false"
HELM_NAMESPACE: default
`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.format.write(&buf, envVars); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestEnvFormatWriteSingle(t *testing.T) {
	tests := []struct {
		name   string
		format envFormat
		want   string
	}{{
		name:   "key=value format prints the bare value",
		format: envFormatKeyValue,
		want:   "default\n",
	}, {
		name:   "json format",
		format: envFormatJSON,
		want:   `{"HELM_NAMESPACE":"default"}` + "\n",
	}, {
		name:   "yaml format",
		format: envFormatYAML,
		want:   "HELM_NAMESPACE: default\n",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.format.writeSingle(&buf, "HELM_NAMESPACE", "default"); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestEnvFileCompletion(t *testing.T) {
	checkFileCompletion(t, "env", false)
	checkFileCompletion(t, "env HELM_BIN", false)
}
