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

package chart

import (
	"testing"

	v3 "helm.sh/helm/v4/internal/chart/v3"
)

// A v3 chart that declares a dependency in Chart.yaml but has not vendored it
// under charts/ (the normal state before "helm dependency build") must not panic
// when its metadata dependencies are read.
func TestV3AccessorMetaDependenciesUnvendored(t *testing.T) {
	c := &v3.Chart{Metadata: &v3.Metadata{
		APIVersion:   "v3",
		Name:         "x",
		Version:      "1.0.0",
		Dependencies: []*v3.Dependency{{Name: "foo", Version: "1.0.0", Repository: "https://example.com"}},
	}}
	acc, err := NewAccessor(c)
	if err != nil {
		t.Fatal(err)
	}
	deps := acc.MetaDependencies()
	if len(deps) != 1 {
		t.Fatalf("expected 1 meta dependency, got %d", len(deps))
	}
}
