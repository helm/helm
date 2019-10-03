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
	"io/ioutil"
	"testing"

	"helm.sh/helm/v3/internal/experimental/registry"
)

func chartSaveAction(t *testing.T) *ChartSave {
	t.Helper()
	config := actionConfigFixture(t)
	action := NewChartSave(config)
	return action
}

func TestChartSave(t *testing.T) {
	action := chartSaveAction(t)

	input := buildChart()
	if err := action.Run(ioutil.Discard, input, "localhost:5000/test:0.2.0"); err != nil {
		t.Error(err)
	}

	ref, err := registry.ParseReference("localhost:5000/test:0.2.0")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := action.cfg.RegistryClient.LoadChart(ref); err != nil {
		t.Error(err)
	}

	// now let's check if `helm chart save` can use the chart version when the tag is not present
	if err := action.Run(ioutil.Discard, input, "localhost:5000/test"); err != nil {
		t.Error(err)
	}

	ref, err = registry.ParseReference("localhost:5000/test")
	if err != nil {
		t.Fatal(err)
	}

	// TODO: guess latest based on semver?
	_, err = action.cfg.RegistryClient.LoadChart(ref)
	if err == nil {
		t.Error("Expected error parsing ref without tag")
	}

	ref.Tag = "0.1.0"
	if _, err := action.cfg.RegistryClient.LoadChart(ref); err != nil {
		t.Error(err)
	}
}
