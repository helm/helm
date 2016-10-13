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
	"os"
	"strings"
	"testing"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestUpdateCmd(t *testing.T) {
	thome, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	oldhome := homePath()
	helmHome = thome
	defer func() {
		helmHome = oldhome
		os.Remove(thome)
	}()

	out := bytes.NewBuffer(nil)
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.Entry, verbose bool, out io.Writer, home helmpath.Home) {
		for _, re := range repos {
			fmt.Fprintln(out, re.Name)
		}
	}
	uc := &repoUpdateCmd{
		out:    out,
		update: updater,
		home:   helmpath.Home(thome),
	}
	if err := uc.run(); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); !strings.Contains(got, "charts") || !strings.Contains(got, "local") {
		t.Errorf("Expected 'charts' and 'local' (in any order) got %q", got)
	}
}

func TestUpdateCharts(t *testing.T) {
	srv, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	oldhome := homePath()
	helmHome = thome
	defer func() {
		srv.Stop()
		helmHome = oldhome
		os.Remove(thome)
	}()
	if err := ensureTestHome(helmpath.Home(thome), t); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)
	repos := []*repo.Entry{
		{Name: "charts", URL: srv.URL()},
	}
	updateCharts(repos, false, buf, helmpath.Home(thome))

	got := buf.String()
	if strings.Contains(got, "Unable to get an update") {
		t.Errorf("Failed to get a repo: %q", got)
	}
	if !strings.Contains(got, "Update Complete.") {
		t.Errorf("Update was not successful")
	}
}
