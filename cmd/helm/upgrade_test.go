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
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
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
	ch, _ = chartutil.Load(chartPath)

	tests := []releaseCase{
		{
			name:     "upgrade a release",
			args:     []string{"funny-bunny", chartPath},
			resp:     releaseMock(&releaseOptions{name: "funny-bunny", version: 2, chart: ch}),
			expected: "It's not you. It's me\nYour upgrade looks valid but this command is still under active development.\nHang tight.\n",
		},
	}

	cmd := func(c *fakeReleaseClient, out io.Writer) *cobra.Command {
		return newUpgradeCmd(c, out)
	}

	runReleaseCases(t, tests, cmd)

}
