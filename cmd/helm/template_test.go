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
	fixturePath  = "testdata/testcharts/chart-verify-templates"
)

func TestTemplateCmd(t *testing.T) {
	subchart1AbsChartPath, err := filepath.Abs(subchart1ChartPath)
	fixtureAbsPath, err := filepath.Abs(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		desc        string
		args        []string
		expectValue string
		expectError string
	}{
		{
			name:        "check_name",
			desc:        "check for a known name in chart",
			args:        []string{subchart1ChartPath, "--test-outfile", filepath.Join(fixtureAbsPath, "subpop_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_set_name",
			desc:        "verify --set values exist",
			args:        []string{subchart1ChartPath, "-x", "templates/service.yaml", "--set", "service.name=apache", "--test-outfile", filepath.Join(fixturePath, "subpop_subchart1_service.yaml")},
			expectValue: "verification passed",
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
			args:        []string{subchart1ChartPath, "-x", filepath.Join(subchart1AbsChartPath, "templates", "service.yaml"), "--set", "service.name=apache", "--test-outfile", filepath.Join(fixturePath, "subpop_subchart1_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_execute_subchart_template",
			desc:        "verify --execute single template on a subchart template",
			args:        []string{subchart1ChartPath, "-x", "charts/subcharta/templates/service.yaml", "--set", "subcharta.service.name=foobar", "--test-outfile", filepath.Join(fixturePath, "subpop_subcharta_foobar_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_execute_subchart_template_for_tgz_subchart",
			desc:        "verify --execute single template on a subchart template where the subchart is a .tgz in the chart directory",
			args:        []string{frobnitzChartPath, "-x", "charts/mariner/templates/placeholder.tpl", "--set", "mariner.name=moon", "--test-outfile", filepath.Join(fixturePath, "mariner.yaml")},
			expectValue: "Goodbye moon",
		},
		{
			name:        "check_namespace",
			desc:        "verify --namespace",
			args:        []string{subchart1ChartPath, "--namespace", "test", "--test-outfile", filepath.Join(fixturePath, "subpop_set-namespace_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_release_name",
			desc:        "verify --release exists",
			args:        []string{subchart1ChartPath, "--name", "test", "--test-outfile", filepath.Join(fixturePath, "subpop_set-release_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_invalid_name_uppercase",
			desc:        "verify the release name using capitals is invalid",
			args:        []string{subchart1ChartPath, "--name", "FOO"},
			expectError: "is invalid",
		},
		{
			name:        "check_release_name_periods",
			desc:        "verify the release name using periods is valid",
			args:        []string{subchart1ChartPath, "--name", "foo.bar", "--test-outfile", filepath.Join(fixturePath, "subpop_set-release-periods_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_invalid_name_uppercase",
			desc:        "verify the release name using underscores is invalid",
			args:        []string{subchart1ChartPath, "--name", "foo_bar"},
			expectError: "is invalid",
		},
		{
			name:        "check_release_is_install",
			desc:        "verify --is-upgrade toggles .Release.IsInstall",
			args:        []string{subchart1ChartPath, "--is-upgrade=false", "--test-outfile", filepath.Join(fixturePath, "subpop_set-release-install_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_release_is_upgrade",
			desc:        "verify --is-upgrade toggles .Release.IsUpgrade",
			args:        []string{subchart1ChartPath, "--is-upgrade", "true", "--test-outfile", filepath.Join(fixturePath, "subpop_set-release-upgrade_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_notes",
			desc:        "verify --notes shows notes",
			args:        []string{subchart1ChartPath, "--notes", "true", "--test-outfile", filepath.Join(fixturePath, "subpop_subchart1_NOTES.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_values_files",
			desc:        "verify --values files values exist",
			args:        []string{subchart1ChartPath, "--values", subchart1ChartPath + "/charts/subchartA/values.yaml", "--test-outfile", filepath.Join(fixturePath, "subpop_values_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_invalid_name_template",
			desc:        "verify the relase name generate by template is invalid",
			args:        []string{subchart1ChartPath, "--name-template", "foobar-{{ b64enc \"abc\" }}-baz"},
			expectError: "is invalid",
		},
		{
			name:        "check_name_template",
			desc:        "verify --name-template result exists",
			args:        []string{subchart1ChartPath, "--name-template", "foobar-{{ lower \"ABC\" }}-baz", "--test-outfile", filepath.Join(fixturePath, "subpop_template_service.yaml")},
			expectValue: "verification passed",
		},
		{
			name:        "check_kube_version",
			desc:        "verify --kube-version overrides the kubernetes version",
			args:        []string{subchart1ChartPath, "--kube-version", "1.6", "--test-outfile", filepath.Join(fixturePath, "subpop_kube_service.yaml")},
			expectValue: "verification passed",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// capture stdout
			commandOutput, err := runCommandAndReturnOutput(tt.args)

			if tt.expectError != "" {
				if err == nil {
					fmt.Println(tt.args)
					t.Errorf("expected err: %s, but no error occurred", tt.expectError)
				}
				// non nil error, check if it contains the expected error
				if strings.Contains(err.Error(), tt.expectError) {
					// had the error we were looking for, this test case is
					// done
					return
				}
				fmt.Println(tt.args)
				t.Fatalf("expected err: %q, got: %q", tt.expectError, err)
			} else if err != nil {
				fmt.Println(tt.args)
				t.Errorf("expected no error, got %v", err)
			}


			if !strings.Contains(commandOutput, tt.expectValue) {
				fmt.Println(tt.args)
				t.Errorf("failed to match expected value (%s)\n in\n %s", tt.expectValue, commandOutput)
			}
			buf.Reset()
		})
	}
}

func runCommandAndReturnOutput(args []string) (string, error) {
	// capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	// execute template command
	out := bytes.NewBuffer(nil)
	cmd := newTemplateCmd(out)
	cmd.SetArgs(args)
	err := cmd.Execute()

	// restore stdout
	w.Close()
	os.Stdout = old
	var b bytes.Buffer
	io.Copy(&b, r)
	r.Close()

	commandOutput := b.String()
	return commandOutput, err
}
