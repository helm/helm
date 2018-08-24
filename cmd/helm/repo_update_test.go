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

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestUpdateCmd(t *testing.T) {
	defer resetEnv()()

	hh := testHelmHome(t)
	settings.Home = hh

	out := bytes.NewBuffer(nil)
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.ChartRepository, out io.Writer, hh helmpath.Home) {
		for _, re := range repos {
			fmt.Fprintln(out, re.Config.Name)
		}
	}
	o := &repoUpdateOptions{
		update: updater,
		home:   hh,
	}
	if err := o.run(out); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); !strings.Contains(got, "charts") {
		t.Errorf("Expected 'charts' got %q", got)
	}
}

func TestUpdateCharts(t *testing.T) {
	defer resetEnv()()

	ts, hh, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		ts.Stop()
		os.RemoveAll(hh.String())
	}()
	ensureTestHome(t, hh)
	settings.Home = hh

	r, err := repo.NewChartRepository(&repo.Entry{
		Name:  "charts",
		URL:   ts.URL(),
		Cache: hh.CacheIndex("charts"),
	}, getter.All(settings))
	if err != nil {
		t.Error(err)
	}

	b := bytes.NewBuffer(nil)
	updateCharts([]*repo.ChartRepository{r}, b, hh)

	got := b.String()
	if strings.Contains(got, "Unable to get an update") {
		t.Errorf("Failed to get a repo: %q", got)
	}
	if !strings.Contains(got, "Update Complete.") {
		t.Error("Update was not successful")
	}
}
