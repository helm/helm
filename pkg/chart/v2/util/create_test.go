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

package util

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

func TestCreate(t *testing.T) {
	tdir := t.TempDir()

	c, err := Create("foo", tdir)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	mychart, err := loader.LoadDir(c)
	if err != nil {
		t.Fatalf("Failed to load newly created chart %q: %s", c, err)
	}

	if mychart.Name() != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", mychart.Name())
	}

	for _, f := range []string{
		ChartfileName,
		DeploymentName,
		HelpersName,
		IgnorefileName,
		NotesName,
		ServiceAccountName,
		ServiceName,
		TemplatesDir,
		TemplatesTestsDir,
		TestConnectionName,
		ValuesfileName,
	} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("Expected %s file: %s", f, err)
		}
	}
}

func TestCreateFrom(t *testing.T) {
	tdir := t.TempDir()

	cf := &chart.Metadata{
		APIVersion: chart.APIVersionV1,
		Name:       "foo",
		Version:    "0.1.0",
	}
	srcdir := "./testdata/frobnitz/charts/mariner"

	if err := CreateFrom(cf, tdir, srcdir); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")
	c := filepath.Join(tdir, cf.Name)
	mychart, err := loader.LoadDir(c)
	if err != nil {
		t.Fatalf("Failed to load newly created chart %q: %s", c, err)
	}

	if mychart.Name() != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", mychart.Name())
	}

	for _, f := range []string{
		ChartfileName,
		ValuesfileName,
		filepath.Join(TemplatesDir, "placeholder.tpl"),
	} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("Expected %s file: %s", f, err)
		}

		// Check each file to make sure <CHARTNAME> has been replaced
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("Unable to read file %s: %s", f, err)
		}
		if bytes.Contains(b, []byte("<CHARTNAME>")) {
			t.Errorf("File %s contains <CHARTNAME>", f)
		}
	}

	cf.Name = "Bad.Chart"
	if err := CreateFrom(cf, t.TempDir(), srcdir); err == nil {
		t.Fatal("CreateFrom with invalid chart name returned no error")
	} else if err.Error() != validChartNameMessage {
		t.Fatalf("CreateFrom returned %q, want %q", err, validChartNameMessage)
	}
}

// TestCreate_Overwrite is a regression test for making sure that files are overwritten.
func TestCreate_Overwrite(t *testing.T) {
	tdir := t.TempDir()

	var errlog bytes.Buffer

	if _, err := Create("foo", tdir); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	tplname := filepath.Join(dir, "templates/hpa.yaml")
	writeFile(tplname, []byte("FOO"))

	// Now re-run the create
	Stderr = &errlog
	if _, err := Create("foo", tdir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(tplname)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) == "FOO" {
		t.Fatal("File that should have been modified was not.")
	}

	if errlog.Len() == 0 {
		t.Error("Expected warnings about overwriting files.")
	}
}

func TestValidateChartName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{
			name:    "",
			wantErr: "chart name must be between 1 and 250 characters",
		},
		{
			name: "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name: "bad-chart-1",
		},
		{
			name: "bad--chart",
		},
		{
			name:    "BadChart",
			wantErr: validChartNameMessage,
		},
		{
			name:    "bad.chart",
			wantErr: validChartNameMessage,
		},
		{
			name:    "bad_chart",
			wantErr: validChartNameMessage,
		},
		{
			name:    "-badchart",
			wantErr: validChartNameMessage,
		},
		{
			name:    "badchart-",
			wantErr: validChartNameMessage,
		},
		{
			name:    "$hello",
			wantErr: validChartNameMessage,
		},
		{
			name:    "Hellô",
			wantErr: validChartNameMessage,
		},
		{
			name:    "he%%o",
			wantErr: validChartNameMessage,
		},
		{
			name:    "he\nllo",
			wantErr: validChartNameMessage,
		},
		{
			name: "abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"abcdefghijklmnopqrstuvwxyz-_." +
				"ABCDEFGHIJKLMNOPQRSTUVWXYZ-_.",
			wantErr: "chart name must be between 1 and 250 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChartName(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateChartName(%q) returned unexpected error: %s", tt.name, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateChartName(%q) returned no error, want %q", tt.name, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("ValidateChartName(%q) returned %q, want %q", tt.name, err, tt.wantErr)
			}
		})
	}
}
