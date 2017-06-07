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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestUpgradeCmd(t *testing.T) {
	tmpChart, _ := ioutil.TempDir("testdata", "tmp")
	defer os.RemoveAll(tmpChart)
	cfile := &chart.Metadata{
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.0",
	}
	chartPath, err := chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Errorf("Error creating chart for upgrade: %v", err)
	}
	ch, _ := chartutil.Load(chartPath)
	_ = releaseMock(&releaseOptions{
		name:  "funny-bunny",
		chart: ch,
	})

	// update chart version
	cfile = &chart.Metadata{
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.2",
	}

	chartPath, err = chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Errorf("Error creating chart: %v", err)
	}
	ch, err = chartutil.Load(chartPath)
	if err != nil {
		t.Errorf("Error loading updated chart: %v", err)
	}

	// update chart version again
	cfile = &chart.Metadata{
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.3",
	}

	chartPath, err = chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Errorf("Error creating chart: %v", err)
	}
	var ch2 *chart.Chart
	ch2, err = chartutil.Load(chartPath)
	if err != nil {
		t.Errorf("Error loading updated chart: %v", err)
	}

	originalDepsPath := filepath.Join("testdata/testcharts/reqtest")
	missingDepsPath := filepath.Join("testdata/testcharts/chart-missing-deps")
	badDepsPath := filepath.Join("testdata/testcharts/chart-bad-requirements")
	var ch3 *chart.Chart
	ch3, err = chartutil.Load(originalDepsPath)
	if err != nil {
		t.Errorf("Error loading chart with missing dependencies: %v", err)
	}

	tests := []releaseCase{
		{
			name:     "upgrade a release",
			args:     []string{"funny-bunny", chartPath},
			resp:     releaseMock(&releaseOptions{name: "funny-bunny", version: 2, chart: ch}),
			expected: "Release \"funny-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "upgrade a release with timeout",
			args:     []string{"funny-bunny", chartPath},
			flags:    []string{"--timeout", "120"},
			resp:     releaseMock(&releaseOptions{name: "funny-bunny", version: 3, chart: ch2}),
			expected: "Release \"funny-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "upgrade a release with --reset-values",
			args:     []string{"funny-bunny", chartPath},
			flags:    []string{"--reset-values", "true"},
			resp:     releaseMock(&releaseOptions{name: "funny-bunny", version: 4, chart: ch2}),
			expected: "Release \"funny-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "upgrade a release with --reuse-values",
			args:     []string{"funny-bunny", chartPath},
			flags:    []string{"--reuse-values", "true"},
			resp:     releaseMock(&releaseOptions{name: "funny-bunny", version: 5, chart: ch2}),
			expected: "Release \"funny-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "install a release with 'upgrade --install'",
			args:     []string{"zany-bunny", chartPath},
			flags:    []string{"-i"},
			resp:     releaseMock(&releaseOptions{name: "zany-bunny", version: 1, chart: ch}),
			expected: "Release \"zany-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "install a release with 'upgrade --install' and timeout",
			args:     []string{"crazy-bunny", chartPath},
			flags:    []string{"-i", "--timeout", "120"},
			resp:     releaseMock(&releaseOptions{name: "crazy-bunny", version: 1, chart: ch}),
			expected: "Release \"crazy-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name:     "upgrade a release with wait",
			args:     []string{"crazy-bunny", chartPath},
			flags:    []string{"--wait"},
			resp:     releaseMock(&releaseOptions{name: "crazy-bunny", version: 2, chart: ch2}),
			expected: "Release \"crazy-bunny\" has been upgraded. Happy Helming!\n",
		},
		{
			name: "upgrade a release with missing dependencies",
			args: []string{"bonkers-bunny", missingDepsPath},
			resp: releaseMock(&releaseOptions{name: "bonkers-bunny", version: 1, chart: ch3}),
			err:  true,
		},
		{
			name: "upgrade a release with bad dependencies",
			args: []string{"bonkers-bunny", badDepsPath},
			resp: releaseMock(&releaseOptions{name: "bonkers-bunny", version: 1, chart: ch3}),
			err:  true,
		},
	}

	cmd := func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newUpgradeCmd(c, out)
	}

	runReleaseCases(t, tests, cmd)

}
