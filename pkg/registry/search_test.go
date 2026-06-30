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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestSearchOptVersion(t *testing.T) {
	op := &searchOperation{}
	SearchOptVersion(">=1.0.0")(op)
	assert.Equal(t, ">=1.0.0", op.version)
}

func TestSearchOptVersions(t *testing.T) {
	op := &searchOperation{}
	SearchOptVersions(true)(op)
	assert.True(t, op.versions)
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		Charts: []*SearchResultChart{
			{
				Reference: "oci://ghcr.io/org/charts/mychart",
				Chart: &chart.Metadata{
					Name:        "mychart",
					Version:     "1.2.0",
					AppVersion:  "2.0.0",
					Description: "A test chart",
				},
			},
			{
				Reference: "oci://ghcr.io/org/charts/mychart",
				Chart: &chart.Metadata{
					Name:        "mychart",
					Version:     "1.1.0",
					AppVersion:  "1.9.0",
					Description: "A test chart",
				},
			},
		},
	}

	assert.Equal(t, 2, len(result.Charts))
	assert.Equal(t, "1.2.0", result.Charts[0].Chart.Version)
	assert.Equal(t, "1.1.0", result.Charts[1].Chart.Version)
	assert.Equal(t, "oci://ghcr.io/org/charts/mychart", result.Charts[0].Reference)
}
