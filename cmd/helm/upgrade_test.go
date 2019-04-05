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

package main

import (
	"fmt"
	"testing"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/release"
)

func TestUpgradeCmd(t *testing.T) {
	tmpChart := testTempDir(t)
	cfile := &chart.Metadata{
		APIVersion:  chart.APIVersionV1,
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.0",
	}
	chartPath, err := chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Fatalf("Error creating chart for upgrade: %v", err)
	}
	ch, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("Error loading chart: %v", err)
	}
	_ = release.Mock(&release.MockReleaseOptions{
		Name:  "funny-bunny",
		Chart: ch,
	})

	// update chart version
	cfile = &chart.Metadata{
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.2",
	}

	chartPath, err = chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Fatalf("Error creating chart: %v", err)
	}
	ch, err = loader.Load(chartPath)
	if err != nil {
		t.Fatalf("Error loading updated chart: %v", err)
	}

	// update chart version again
	cfile = &chart.Metadata{
		Name:        "testUpgradeChart",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.3",
	}

	chartPath, err = chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Fatalf("Error creating chart: %v", err)
	}
	var ch2 *chart.Chart
	ch2, err = loader.Load(chartPath)
	if err != nil {
		t.Fatalf("Error loading updated chart: %v", err)
	}

	missingDepsPath := "testdata/testcharts/chart-missing-deps"
	badDepsPath := "testdata/testcharts/chart-bad-requirements"

	relMock := func(n string, v int, ch *chart.Chart) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch})
	}

	tests := []cmdTestCase{
		{
			name:   "upgrade a release",
			cmd:    fmt.Sprintf("upgrade funny-bunny '%s'", chartPath),
			golden: "output/upgrade.txt",
			rels:   []*release.Release{relMock("funny-bunny", 2, ch)},
		},
		{
			name:   "upgrade a release with timeout",
			cmd:    fmt.Sprintf("upgrade funny-bunny --timeout 120 '%s'", chartPath),
			golden: "output/upgrade-with-timeout.txt",
			rels:   []*release.Release{relMock("funny-bunny", 3, ch2)},
		},
		{
			name:   "upgrade a release with --reset-values",
			cmd:    fmt.Sprintf("upgrade funny-bunny --reset-values '%s'", chartPath),
			golden: "output/upgrade-with-reset-values.txt",
			rels:   []*release.Release{relMock("funny-bunny", 4, ch2)},
		},
		{
			name:   "upgrade a release with --reuse-values",
			cmd:    fmt.Sprintf("upgrade funny-bunny --reuse-values '%s'", chartPath),
			golden: "output/upgrade-with-reset-values2.txt",
			rels:   []*release.Release{relMock("funny-bunny", 5, ch2)},
		},
		{
			name:   "install a release with 'upgrade --install'",
			cmd:    fmt.Sprintf("upgrade zany-bunny -i '%s'", chartPath),
			golden: "output/upgrade-with-install.txt",
			rels:   []*release.Release{relMock("zany-bunny", 1, ch)},
		},
		{
			name:   "install a release with 'upgrade --install' and timeout",
			cmd:    fmt.Sprintf("upgrade crazy-bunny -i --timeout 120 '%s'", chartPath),
			golden: "output/upgrade-with-install-timeout.txt",
			rels:   []*release.Release{relMock("crazy-bunny", 1, ch)},
		},
		{
			name:   "upgrade a release with wait",
			cmd:    fmt.Sprintf("upgrade crazy-bunny --wait '%s'", chartPath),
			golden: "output/upgrade-with-wait.txt",
			rels:   []*release.Release{relMock("crazy-bunny", 2, ch2)},
		},
		{
			name:      "upgrade a release with missing dependencies",
			cmd:       fmt.Sprintf("upgrade bonkers-bunny%s", missingDepsPath),
			golden:    "output/upgrade-with-missing-dependencies.txt",
			wantError: true,
		},
		{
			name:      "upgrade a release with bad dependencies",
			cmd:       fmt.Sprintf("upgrade bonkers-bunny '%s'", badDepsPath),
			golden:    "output/upgrade-with-bad-dependencies.txt",
			wantError: true,
		},
	}
	runTestCmd(t, tests)
}
