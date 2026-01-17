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
	"time"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

func TestShow(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "README.md", ModTime: modTime, Data: []byte("README\n")},
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
			{Name: "crds/baz.yaml", ModTime: modTime, Data: []byte("baz\n")},
		},
		Raw: []*common.File{
			{Name: "values.yaml", ModTime: modTime, Data: []byte("VALUES\n")},
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

---
baz

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoValues(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
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
	config := actionConfigFixture(t)
	client := NewShow(ShowValues, config)
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
	config := actionConfigFixture(t)
	client := NewShow(ShowCRDs, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
			{Name: "crds/baz.yaml", ModTime: modTime, Data: []byte("baz\n")},
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

---
baz

`
	if output != expect {
		t.Errorf("Expected\n%q\nGot\n%q\n", expect, output)
	}
}

func TestShowNoReadme(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)
	modTime := time.Now()
	client.chart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "alpine"},
		Files: []*common.File{
			{Name: "crds/ignoreme.txt", ModTime: modTime, Data: []byte("error")},
			{Name: "crds/foo.yaml", ModTime: modTime, Data: []byte("---\nfoo\n")},
			{Name: "crds/bar.json", ModTime: modTime, Data: []byte("---\nbar\n")},
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

func TestShowSetRegistryClient(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewShow(ShowAll, config)

	registryClient := &registry.Client{}
	client.SetRegistryClient(registryClient)
	assert.Equal(t, registryClient, client.registryClient)
}
