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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCmd(t *testing.T) {
	out := bytes.NewBuffer(nil)
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos map[string]string, verbose bool, out io.Writer) {
		for name := range repos {
			fmt.Fprintln(out, name)
		}
	}
	uc := &repoUpdateCmd{
		out:      out,
		update:   updater,
		repoFile: "testdata/repositories.yaml",
	}
	uc.run()

	if got := out.String(); !strings.Contains(got, "charts") || !strings.Contains(got, "local") {
		t.Errorf("Expected 'charts' and 'local' (in any order) got %s", got)
	}
}

const mockRepoIndex = `
mychart-0.1.0:
  name: mychart-0.1.0
  url: localhost:8879/charts/mychart-0.1.0.tgz
  chartfile:
    name: ""
    home: ""
    sources: []
    version: ""
    description: ""
    keywords: []
    maintainers: []
    engine: ""
    icon: ""
`

func TestUpdateCharts(t *testing.T) {
	// This tests the repo in isolation. It creates a mock HTTP server that simply
	// returns a static YAML file in the anticipate format.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mockRepoIndex))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	buf := bytes.NewBuffer(nil)
	repos := map[string]string{
		"charts": srv.URL,
	}
	updateCharts(repos, false, buf)

	got := buf.String()
	if strings.Contains(got, "Unable to get an update") {
		t.Errorf("Failed to get a repo: %q", got)
	}
	if !strings.Contains(got, "Update Complete.") {
		t.Errorf("Update was not successful")
	}
}
