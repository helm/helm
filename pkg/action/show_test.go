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

package action

import (
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

func TestShow(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShowWithConfig(ShowAll, config)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*chart.File{
			{Name: "README.md", Data: []byte("README\n")},
			{Name: "crds/ignoreme.txt", Data: []byte("error")},
			{Name: "crds/foo.yaml", Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", Data: []byte("---\nbar\n")},
		},
		Raw: []*chart.File{
			{Name: "values.yaml", Data: []byte("VALUES\n")},
		},
		Values: map[string]interface{}{},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `name: alpine

---
VALUES

---
README

---
foo

---
bar

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoValues(t *testing.T) {
	client := NewShow(ShowAll)
	client.chart = new(chart.Chart)

	// Regression tests for missing values. See issue #1024.
	client.OutputFormat = ShowValues
	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	if len(output) != 0 {
		t.Errorf("expected empty values buffer, got %s", output)
	}
}

func TestShowValuesByJsonPathFormat(t *testing.T) {
	client := NewShow(ShowValues)
	client.JSONPathTemplate = "{$.nestedKey.simpleKey}"
	client.chart = buildChart(withSampleValues())
	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}
	expect := "simpleValue"
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowCRDs(t *testing.T) {
	client := NewShow(ShowCRDs)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*chart.File{
			{Name: "crds/ignoreme.txt", Data: []byte("error")},
			{Name: "crds/foo.yaml", Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", Data: []byte("---\nbar\n")},
		},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `---
foo

---
bar

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoReadme(t *testing.T) {
	client := NewShow(ShowAll)
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*chart.File{
			{Name: "crds/ignoreme.txt", Data: []byte("error")},
			{Name: "crds/foo.yaml", Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", Data: []byte("---\nbar\n")},
		},
	}

	output, err := client.Run("")
	if err != nil {
		t.Fatal(err)
	}

	expect := `name: alpine

---
foo

---
bar

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}
