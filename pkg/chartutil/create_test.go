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

package chartutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

func checkDir(t *testing.T, d string) {
	if fi, err := os.Stat(d); err != nil {
		t.Errorf("Expected %s dir: %s", d, err)
	} else if !fi.IsDir() {
		t.Errorf("Expected %s to be a directory.", d)
	}
}

func checkFile(t *testing.T, f string) {
	if fi, err := os.Stat(f); err != nil {
		t.Errorf("Expected %s file: %s", filepath.Base(f), err)
	} else if fi.IsDir() {
		t.Errorf("Expected %s to be a file.", filepath.Base(f))
	}
}

func TestCreate(t *testing.T) {
	tdir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	cf := &chart.Metadata{Name: "foo"}

	c, err := Create(cf, tdir)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	mychart, err := LoadDir(c)
	if err != nil {
		t.Fatalf("Failed to load newly created chart %q: %s", c, err)
	}

	if mychart.Metadata.Name != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", mychart.Metadata.Name)
	}

	for _, d := range []string{TemplatesDir, ChartsDir} {
		checkDir(t, filepath.Join(dir, d))
	}

	for _, f := range []string{ChartfileName, ValuesfileName, IgnorefileName} {
		checkFile(t, filepath.Join(dir, f))
	}

	for _, f := range []string{NotesName, DeploymentName, ServiceName, HelpersName} {
		checkFile(t, filepath.Join(dir, TemplatesDir, f))
	}

	for _, f := range []string{TestConnectionName} {
		checkFile(t, filepath.Join(dir, TemplatesTestsDir, f))
	}

}

func TestCreateFrom(t *testing.T) {
	tdir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	cf := &chart.Metadata{Name: "foo"}
	srcdir := "./testdata/mariner"

	if err := CreateFrom(cf, tdir, srcdir); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	c := filepath.Join(tdir, cf.Name)
	mychart, err := LoadDir(c)
	if err != nil {
		t.Fatalf("Failed to load newly created chart %q: %s", c, err)
	}

	if mychart.Metadata.Name != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", mychart.Metadata.Name)
	}

	for _, d := range []string{TemplatesDir, ChartsDir} {
		checkDir(t, filepath.Join(dir, d))
	}

	for _, f := range []string{ChartfileName, ValuesfileName, "requirements.yaml"} {
		checkFile(t, filepath.Join(dir, f))
	}

	for _, f := range []string{"placeholder.tpl"} {
		checkFile(t, filepath.Join(dir, TemplatesDir, f))
	}

	// Ensure we replace `<CHARTNAME>`
	if strings.Contains(mychart.Values.Raw, "<CHARTNAME>") {
		t.Errorf("Did not expect %s to be present in %s", "<CHARTNAME>", mychart.Values.Raw)
	}
}
