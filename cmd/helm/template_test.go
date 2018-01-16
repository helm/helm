/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

var chartPath = "./../../pkg/chartutil/testdata/subpop/charts/subchart1"

func TestTemplateCmd(t *testing.T) {
	absChartPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		desc        string
		args        []string
		expectKey   string
		expectValue string
	}{
		{
			name:        "check_name",
			desc:        "check for a known name in chart",
			args:        []string{chartPath},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: nginx",
		},
		{
			name:        "check_set_name",
			desc:        "verify --set values exist",
			args:        []string{chartPath, "-x", "templates/service.yaml", "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_execute",
			desc:        "verify --execute single template",
			args:        []string{chartPath, "-x", "templates/service.yaml", "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_execute_absolute",
			desc:        "verify --execute single template",
			args:        []string{chartPath, "-x", absChartPath + "/" + "templates/service.yaml", "--set", "service.name=apache"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "protocol: TCP\n    name: apache",
		},
		{
			name:        "check_namespace",
			desc:        "verify --namespace",
			args:        []string{chartPath, "--namespace", "test"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "namespace: \"test\"",
		},
		{
			name:        "check_release_name",
			desc:        "verify --release exists",
			args:        []string{chartPath, "--name", "test"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-name: \"test\"",
		},
		{
			name:        "check_notes",
			desc:        "verify --notes shows notes",
			args:        []string{chartPath, "--notes", "true"},
			expectKey:   "subchart1/templates/NOTES.txt",
			expectValue: "Sample notes for subchart1",
		},
		{
			name:        "check_values_files",
			desc:        "verify --values files values exist",
			args:        []string{chartPath, "--values", chartPath + "/charts/subchartA/values.yaml"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "name: apache",
		},
		{
			name:        "check_name_template",
			desc:        "verify --name-template result exists",
			args:        []string{chartPath, "--name-template", "foobar-{{ b64enc \"abc\" }}-baz"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "release-name: \"foobar-YWJj-baz\"",
		},
		{
			name:        "check_kube_version",
			desc:        "verify --kube-version overrides the kubernetes version",
			args:        []string{chartPath, "--kube-version", "1.6"},
			expectKey:   "subchart1/templates/service.yaml",
			expectValue: "kube-version/major: \"1\"\n    kube-version/minor: \"6\"\n    kube-version/gitversion: \"v1.6.0\"",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		t.Run(tt.name, func(T *testing.T) {
			// capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			// execute template command
			out := bytes.NewBuffer(nil)
			cmd := newTemplateCmd(out)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Errorf("expected: %v, got %v", tt.expectValue, err)
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
