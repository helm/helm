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
	"io/ioutil"
	"strings"
	"testing"
)

func TestShow(t *testing.T) {
	client := NewShow(ShowAll)

	output, err := client.Run("../../cmd/helm/testdata/testcharts/alpine")
	if err != nil {
		t.Fatal(err)
	}

	// Load the data from the textfixture directly.
	cdata, err := ioutil.ReadFile("../../cmd/helm/testdata/testcharts/alpine/Chart.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile("../../cmd/helm/testdata/testcharts/alpine/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	readmeData, err := ioutil.ReadFile("../../cmd/helm/testdata/testcharts/alpine/README.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(output, "---", 3)
	if len(parts) != 3 {
		t.Fatalf("Expected 2 parts, got %d", len(parts))
	}

	expect := []string{
		strings.ReplaceAll(strings.TrimSpace(string(cdata)), "\r", ""),
		strings.ReplaceAll(strings.TrimSpace(string(data)), "\r", ""),
		strings.ReplaceAll(strings.TrimSpace(string(readmeData)), "\r", ""),
	}

	// Problem: ghodss/yaml doesn't marshal into struct order. To solve, we
	// have to carefully craft the Chart.yaml to match.
	for i, got := range parts {
		got = strings.ReplaceAll(strings.TrimSpace(got), "\r", "")
		if got != expect[i] {
			t.Errorf("Expected\n%q\nGot\n%q\n", expect[i], got)
		}
	}

	// Regression tests for missing values. See issue #1024.
	client.OutputFormat = ShowValues
	output, err = client.Run("../../cmd/helm/testdata/testcharts/novals")
	if err != nil {
		t.Fatal(err)
	}

	if len(output) != 0 {
		t.Errorf("expected empty values buffer, got %s", output)
	}
}
