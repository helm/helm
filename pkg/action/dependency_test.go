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
	"bytes"
	"testing"

	"helm.sh/helm/v3/internal/test"
	"helm.sh/helm/v3/pkg/cli/output"
)

func TestList(t *testing.T) {
	for _, tcase := range []struct {
		chart  string
		golden string
		outfmt output.Format
	}{
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies",
			golden: "output/compressed-deps.txt",
			outfmt: output.Table,
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies-2.1.8.tgz",
			golden: "output/compressed-deps-tgz.txt",
			outfmt: output.Table,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies",
			golden: "output/uncompressed-deps.txt",
			outfmt: output.Table,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies-2.1.8.tgz",
			golden: "output/uncompressed-deps-tgz.txt",
			outfmt: output.Table,
		},
		{
			chart:  "testdata/charts/chart-missing-deps",
			golden: "output/missing-deps.txt",
			outfmt: output.Table,
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies",
			golden: "output/compressed-deps.json",
			outfmt: output.JSON,
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies-2.1.8.tgz",
			golden: "output/compressed-deps-tgz.json",
			outfmt: output.JSON,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies",
			golden: "output/uncompressed-deps.json",
			outfmt: output.JSON,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies-2.1.8.tgz",
			golden: "output/uncompressed-deps-tgz.json",
			outfmt: output.JSON,
		},
		{
			chart:  "testdata/charts/chart-missing-deps",
			golden: "output/missing-deps.json",
			outfmt: output.JSON,
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies",
			golden: "output/compressed-deps.yaml",
			outfmt: output.YAML,
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies-2.1.8.tgz",
			golden: "output/compressed-deps-tgz.yaml",
			outfmt: output.YAML,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies",
			golden: "output/uncompressed-deps.yaml",
			outfmt: output.YAML,
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies-2.1.8.tgz",
			golden: "output/uncompressed-deps-tgz.yaml",
			outfmt: output.YAML,
		},
		{
			chart:  "testdata/charts/chart-missing-deps",
			golden: "output/missing-deps.yaml",
			outfmt: output.YAML,
		},
	} {
		buf := bytes.Buffer{}
		if err := tcase.outfmt.Write(&buf, &DependencyListWriter{Chartpath: tcase.chart}); err != nil {
			t.Fatal(err)
		}
		test.AssertGoldenBytes(t, buf.Bytes(), tcase.golden)
	}
}
