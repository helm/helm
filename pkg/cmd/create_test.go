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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	chartv3 "helm.sh/helm/v4/internal/chart/v3"
	chartutilv3 "helm.sh/helm/v4/internal/chart/v3/util"
	"helm.sh/helm/v4/internal/gates"
	"helm.sh/helm/v4/internal/test/ensure"
	chart "helm.sh/helm/v4/pkg/chart"
	chartloader "helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/helmpath"
)

func TestCreateCmd(t *testing.T) {
	t.Chdir(t.TempDir())
	ensure.HelmHome(t)
	cname := "testchart"

	// Run a create
	if _, _, err := executeActionCommand("create " + cname); err != nil {
		t.Fatalf("Failed to run create: %s", err)
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := chartloader.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	acc, err := chart.NewAccessor(c)
	if err != nil {
		t.Fatal(err)
	}

	if acc.Name() != cname {
		t.Errorf("Expected %q name, got %q", cname, acc.Name())
	}
	metadata := acc.MetadataAsMap()
	apiVersion, ok := metadata["APIVersion"].(string)
	if !ok {
		t.Fatal("APIVersion not found in metadata")
	}
	if apiVersion != chartv2.APIVersionV2 {
		t.Errorf("Wrong API version: %q", apiVersion)
	}
}

func TestCreateStarterCmd(t *testing.T) {
	tests := []struct {
		name            string
		chartAPIVersion string
		useAbsolutePath bool
		expectedVersion string
	}{
		{
			name:            "v2 with relative starter path",
			chartAPIVersion: "",
			useAbsolutePath: false,
			expectedVersion: chartv2.APIVersionV2,
		},
		{
			name:            "v2 with absolute starter path",
			chartAPIVersion: "",
			useAbsolutePath: true,
			expectedVersion: chartv2.APIVersionV2,
		},
		{
			name:            "v3 with relative starter path",
			chartAPIVersion: "v3",
			useAbsolutePath: false,
			expectedVersion: chartv3.APIVersionV3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			ensure.HelmHome(t)
			defer resetEnv()()

			// Enable feature gate for v3 charts
			if tt.chartAPIVersion == "v3" {
				t.Setenv(string(gates.ChartV3), "1")
			}

			cname := "testchart"

			// Create a starter using the appropriate chartutil
			starterchart := helmpath.DataPath("starters")
			os.MkdirAll(starterchart, 0o755)
			var err error
			var dest string
			if tt.chartAPIVersion == "v3" {
				dest, err = chartutilv3.Create("starterchart", starterchart)
			} else {
				dest, err = chartutil.Create("starterchart", starterchart)
			}
			if err != nil {
				t.Fatalf("Could not create chart: %s", err)
			}
			t.Logf("Created %s", dest)

			tplpath := filepath.Join(starterchart, "starterchart", "templates", "foo.tpl")
			if err := os.WriteFile(tplpath, []byte("test"), 0o644); err != nil {
				t.Fatalf("Could not write template: %s", err)
			}

			// Build the command
			starterArg := "starterchart"
			if tt.useAbsolutePath {
				starterArg = filepath.Join(starterchart, "starterchart")
			}
			cmd := fmt.Sprintf("create --starter=%s", starterArg)
			if tt.chartAPIVersion == "v3" {
				cmd += fmt.Sprintf(" --chart-api-version=%s", chartv3.APIVersionV3)
			} else {
				cmd += fmt.Sprintf(" --chart-api-version=%s", chartv2.APIVersionV2)
			}
			cmd += " " + cname

			// Run create
			if _, _, err := executeActionCommand(cmd); err != nil {
				t.Fatalf("Failed to run create: %s", err)
			}

			// Test that the chart is there
			if fi, err := os.Stat(cname); err != nil {
				t.Fatalf("no chart directory: %s", err)
			} else if !fi.IsDir() {
				t.Fatalf("chart is not directory")
			}

			// Load and verify the chart
			c, err := chartloader.LoadDir(cname)
			if err != nil {
				t.Fatal(err)
			}

			acc, err := chart.NewAccessor(c)
			if err != nil {
				t.Fatal(err)
			}

			chartName := acc.Name()
			metadata := acc.MetadataAsMap()
			apiVersion, ok := metadata["APIVersion"].(string)
			if !ok {
				t.Fatal("APIVersion not found in metadata")
			}
			var templates []string
			for _, tpl := range acc.Templates() {
				templates = append(templates, tpl.Name)
			}

			if chartName != cname {
				t.Errorf("Expected %q name, got %q", cname, chartName)
			}
			if apiVersion != tt.expectedVersion {
				t.Errorf("Wrong API version: expected %q, got %q", tt.expectedVersion, apiVersion)
			}

			// Verify custom template exists
			found := false
			for _, name := range templates {
				if name == "templates/foo.tpl" {
					found = true
					break
				}
			}
			if !found {
				t.Error("Did not find foo.tpl")
			}
		})
	}
}

func TestCreateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "create", true)
	checkFileCompletion(t, "create myname", false)
}

func TestCreateCmdChartAPIVersionV2(t *testing.T) {
	t.Chdir(t.TempDir())
	ensure.HelmHome(t)
	cname := "testchart"

	// Run a create with explicit v2
	if _, _, err := executeActionCommand("create --chart-api-version=v2 " + cname); err != nil {
		t.Fatalf("Failed to run create: %s", err)
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := chartloader.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	acc, err := chart.NewAccessor(c)
	if err != nil {
		t.Fatal(err)
	}

	if acc.Name() != cname {
		t.Errorf("Expected %q name, got %q", cname, acc.Name())
	}
	metadata := acc.MetadataAsMap()
	apiVersion, ok := metadata["APIVersion"].(string)
	if !ok {
		t.Fatal("APIVersion not found in metadata")
	}
	if apiVersion != chartv2.APIVersionV2 {
		t.Errorf("Wrong API version: expected %q, got %q", chartv2.APIVersionV2, apiVersion)
	}
}

func TestCreateCmdChartAPIVersionV3(t *testing.T) {
	t.Chdir(t.TempDir())
	ensure.HelmHome(t)
	t.Setenv(string(gates.ChartV3), "1")
	cname := "testchart"

	// Run a create with v3
	if _, _, err := executeActionCommand("create --chart-api-version=v3 " + cname); err != nil {
		t.Fatalf("Failed to run create: %s", err)
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := chartloader.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	acc, err := chart.NewAccessor(c)
	if err != nil {
		t.Fatal(err)
	}

	if acc.Name() != cname {
		t.Errorf("Expected %q name, got %q", cname, acc.Name())
	}
	metadata := acc.MetadataAsMap()
	apiVersion, ok := metadata["APIVersion"].(string)
	if !ok {
		t.Fatal("APIVersion not found in metadata")
	}
	if apiVersion != chartv3.APIVersionV3 {
		t.Errorf("Wrong API version: expected %q, got %q", chartv3.APIVersionV3, apiVersion)
	}
}

func TestCreateCmdInvalidChartAPIVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	ensure.HelmHome(t)
	cname := "testchart"

	// Run a create with invalid version
	_, _, err := executeActionCommand("create --chart-api-version=v1 " + cname)
	if err == nil {
		t.Fatal("Expected error for invalid API version, got nil")
	}

	expectedErr := "unsupported chart API version: v1 (supported: v2, v3)"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}
}
