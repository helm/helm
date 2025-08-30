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

package engine

import (
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzEngineRender(f *testing.F) {
	f.Fuzz(func(_ *testing.T, chartData, valuesBytes []byte) {
		fdp := fuzz.NewConsumer(chartData)
		chrt := &chart.Chart{}
		err := fdp.GenerateStruct(chrt)
		if err != nil {
			return
		}
		values, err := chartutil.ReadValues(valuesBytes)
		if err != nil {
			return
		}
		_, _ = Render(chrt, values)
	})
}

func FuzzEngineFiles(f *testing.F) {
	f.Fuzz(func(_ *testing.T, path, pattern, name, str1, str2, str3, str4, str5 string,
		b1, b2, b3, b4, b5 []byte) {
		files := make(files)
		files[str1] = b1
		files[str2] = b2
		files[str3] = b3
		files[str4] = b4
		files[str5] = b5

		// Test various methods of files
		_ = files.Get(name)
		_ = files.Glob(pattern)
		_ = files.AsConfig()
		_ = files.AsSecrets()
		_ = files.Lines(path)
	})
}
