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
	"testing"
)

var (
	values                       = []byte{}
	namespace                    = "testNamespace"
	strict                       = false
	archivedChartPath            = "testdata/testcharts/compressedchart-0.1.0.tgz"
	archivedChartPathWithHyphens = "testdata/testcharts/compressedchart-with-hyphens-0.1.0.tgz"
	invalidArchivedChartPath     = "testdata/testcharts/invalidcompressedchart0.1.0.tgz"
	chartDirPath                 = "testdata/testcharts/decompressedchart/"
	chartMissingManifest         = "testdata/testcharts/chart-missing-manifest"
)

func TestLintChart(t *testing.T) {
	if _, err := lintChart(chartDirPath, values, namespace, strict); err != nil {
		t.Errorf("%s", err)
	}

	if _, err := lintChart(archivedChartPath, values, namespace, strict); err != nil {
		t.Errorf("%s", err)
	}

	if _, err := lintChart(archivedChartPathWithHyphens, values, namespace, strict); err != nil {
		t.Errorf("%s", err)
	}

	if _, err := lintChart(invalidArchivedChartPath, values, namespace, strict); err == nil {
		t.Errorf("Expected a chart parsing error")
	}

	if _, err := lintChart(chartMissingManifest, values, namespace, strict); err == nil {
		t.Errorf("Expected a chart parsing error")
	}
}
