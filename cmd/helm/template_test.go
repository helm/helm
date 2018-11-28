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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	subchart1ChartPath = "./../../pkg/chartutil/testdata/subpop/charts/subchart1"
	frobnitzChartPath  = "./../../pkg/chartutil/testdata/frobnitz"
)

func TestTemplateCmd(t *testing.T) {
	subchart1AbsChartPath, err := filepath.Abs(subchart1ChartPath)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		desc        string
		args        []string
		expectKey   string
		expectValue string
		expectError string
	}{
		{
			name:        "check_name",
			desc:        "check for a known name in chart",
			args:        []string{subchart1ChartPath},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: nginx",
		},
		{
			name:        "check_set_name",
			desc:        "verify --set values exist",
			args:        []string{subchart1ChartPath, "-x", "templates/service.yaml", "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_execute",
			desc:        "verify --execute single template",
			args:        []string{subchart1ChartPath, "-x", "templates/service.yaml", "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_execute_non_existent",
			desc:        "verify --execute fails on a template that doesn't exist",
			args:        []string{subchart1ChartPath, "-x", "templates/thisdoesn'texist.yaml"},
			expectError: "could not find template",
		},
		{
			name:        "check_execute_absolute",
			desc:        "verify --execute single template",
			args:        []string{subchart1ChartPath, "-x", filepath.Join(subchart1AbsChartPath, "templates", "service.yaml"), "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_execute_subchart_template",
			desc:        "verify --execute single template on a subchart template",
			args:        []string{subchart1ChartPath, "-x", "charts/subcharta/templates/service.yaml", "--set", "subcharta.service.name=foobar"},
			expectKey:   "subchart1/charts/subcharta/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: foobar",
		},
		{
			name:        "check_execute_subchart_template_for_tgz_subchart",
			desc:        "verify --execute single template on a subchart template where the subchart is a .tgz in the chart directory",
			args:        []string{frobnitzChartPath, "-x", "charts/mariner/templates/placeholder.tpl", "--set", "mariner.name=moon"},
			expectKey:   "frobnitz/charts/mariner/templates/placeholder.tpl",
			expectValue: "Goodbye moon",
		},
		{
			name:        "check_namespace",
			desc:        "verify --namespace",
			args:        []string{subchart1ChartPath, "--namespace", "test"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "namespace: \"test\"",
		},
		{
			name:        "check_release_name",
			desc:        "verify --release exists",
			args:        []string{subchart1ChartPath, "--name", "test"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-name: \"test\"",
		},
		{
			name:        "check_invalid_name_uppercase",
			desc:        "verify the release name using capitals is invalid",
			args:        []string{subchart1ChartPath, "--name", "FOO"},
			expectKey:   "subchart1/templates/service.yaml",
			expectError: "is not a valid DNS label",
		},
		{
			name:        "check_invalid_name_uppercase",
			desc:        "verify the release name using periods is invalid",
			args:        []string{subchart1ChartPath, "--name", "foo.bar"},
			expectKey:   "subchart1/templates/service.yaml",
			expectError: "is not a valid DNS label",
		},
		{
			name:        "check_invalid_name_uppercase",
			desc:        "verify the release name using underscores is invalid",
			args:        []string{subchart1ChartPath, "--name", "foo_bar"},
			expectKey:   "subchart1/templates/service.yaml",
			expectError: "is not a valid DNS label",
		},
		{
			name:        "check_release_is_install",
			desc:        "verify --is-upgrade toggles .Release.IsInstall",
			args:        []string{subchart1ChartPath, "--is-upgrade=false"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-is-install: \"true\"",
		},
		{
			name:        "check_release_is_upgrade",
			desc:        "verify --is-upgrade toggles .Release.IsUpgrade",
			args:        []string{subchart1ChartPath, "--is-upgrade", "true"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-is-upgrade: \"true\"",
		},
		{
			name:        "check_notes",
			desc:        "verify --notes shows notes",
			args:        []string{subchart1ChartPath, "--notes", "true"},
			expectKey:   "subchart1/templates/NOTES.txt",
			expectValue: "Sample notes for subchart1",
		},
		{
			name:        "check_values_files",
			desc:        "verify --values files values exist",
			args:        []string{subchart1ChartPath, "--values", subchart1ChartPath + "/charts/subchartA/values.yaml"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "name: apache",
		},
		{
			name:        "check_invalid_name_template",
			desc:        "verify the relase name generate by template is invalid",
			args:        []string{subchart1ChartPath, "--name-template", "foobar-{{ b64enc \"abc\" }}-baz"},
			expectError: "is not a valid DNS label",
		},
		{
			name:        "check_name_template",
			desc:        "verify --name-template result exists",
			args:        []string{subchart1ChartPath, "--name-template", "foobar-{{ lower \"ABC\" }}-baz"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-name: \"foobar-abc-baz\"",
		},
		{
			name:        "check_kube_version",
			desc:        "verify --kube-version overrides the kubernetes version",
			args:        []string{subchart1ChartPath, "--kube-version", "1.6"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "kube-version/major: \"1\"\n    kube-version/minor: \"6\"\n    kube-version/gitversion: \"v1.6.0\"",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			// execute template command
			out := bytes.NewBuffer(nil)
			cmd := newTemplateCmd(out)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectError != "" {
				if err == nil {
					t.Errorf("expected err: %s, but no error occurred", tt.expectError)
				}
				// non nil error, check if it contains the expected error
				if strings.Contains(err.Error(), tt.expectError) {
					// had the error we were looking for, this test case is
					// done
					return
				}
				t.Fatalf("expected err: %q, got: %q", tt.expectError, err)
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			// restore stdout
			w.Close()
			os.Stdout = old
			var b bytes.Buffer
			io.Copy(&b, r)
			r.Close()
			// scan yaml into map[<path>]yaml
			scanner := bufio.NewScanner(&b)
			next := false
			lastKey := ""
			m := map[string]string{}
			for scanner.Scan() {
				if scanner.Text() == "---" {
					next = true
				} else if next {
					// remove '# Source: '
					head := "# Source: "
					lastKey = scanner.Text()[len(head):]
					next = false
				} else {
					m[lastKey] = m[lastKey] + scanner.Text() + "\n"
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading standard input:", err)
			}
			if v, ok := m[tt.expectKey]; ok {
				if !strings.Contains(v, tt.expectValue) {
					t.Errorf("failed to match expected value %s in %s", tt.expectValue, v)
				}
			} else {
				t.Errorf("could not find key %s", tt.expectKey)
			}
			buf.Reset()
		})
	}
}
