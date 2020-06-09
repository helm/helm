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
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

var StubManifest = `apiVersion: v1
kind: Secret
metadata:
  name: fixture
`

func TestGetDependencies(t *testing.T) {
	chartWithDependencies := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "prims47",
			Version:    "4.7.7-beta.1",
			AppVersion: "1.0",
			Dependencies: []*chart.Dependency{
				{Name: "fuego", Version: "7.7.0", Repository: "https://prims47-fuego.com"},
				{Name: "pepito", Version: "4.2.7'", Repository: "https://pepito.com"},
			},
		},
		Templates: []*chart.File{
			{Name: "templates/get-dependencies.tpl", Data: []byte(StubManifest)},
		},
	}

	tests := []cmdTestCase{{
		name:   "get dependencies with release (has dependencies)",
		cmd:    "get dependencies prims47",
		golden: "output/get-dependencies.txt",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name:  "prims47",
				Chart: chartWithDependencies,
			}),
		},
	}, {
		name:   "get dependencies with release (has not dependencies)",
		cmd:    "get dependencies prims47",
		golden: "output/get-dependencies-empty.txt",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name: "prims47",
			}),
		},
	}, {
		name:      "get dependencies without args",
		cmd:       "get dependencies",
		golden:    "output/get-dependencies-no-args.txt",
		wantError: true,
	}, {
		name:   "get dependencies with release (has dependencies) to json",
		cmd:    "get dependencies prims47 --output json",
		golden: "output/get-dependencies.json",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name:  "prims47",
				Chart: chartWithDependencies,
			}),
		},
	}, {
		name:   "get dependencies with release (has not dependencies) to json",
		cmd:    "get dependencies prims47 --output json",
		golden: "output/get-dependencies-empty.json",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name: "prims47",
			}),
		},
	}, {
		name:   "get dependencies with release (has dependencies) to yaml",
		cmd:    "get dependencies prims47 --output yaml",
		golden: "output/get-dependencies.yaml",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name:  "prims47",
				Chart: chartWithDependencies,
			}),
		},
	}, {
		name:   "get dependencies with release (has not dependencies) to yaml",
		cmd:    "get dependencies prims47 --output yaml",
		golden: "output/get-dependencies-empty.yaml",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{
				Name: "prims47",
			}),
		},
	}}

	runTestCmd(t, tests)
}

func TestGetDependenciesRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get dependencies")
}
