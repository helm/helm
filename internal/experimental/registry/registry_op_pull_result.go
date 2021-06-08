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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"helm.sh/helm/v3/pkg/chart"
)

type (
	// PullResult is the result returned upon successful pull.
	PullResult struct {
		Manifest *descriptorPullSummary         `json:"manifest"`
		Config   *descriptorPullSummary         `json:"config"`
		Chart    *descriptorPullSummaryWithMeta `json:"chart"`
		Prov     *descriptorPullSummary         `json:"prov"`
		Ref      string                         `json:"ref"`
	}

	descriptorPullSummary struct {
		Data   []byte `json:"-"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	}

	descriptorPullSummaryWithMeta struct {
		descriptorPullSummary
		Meta *chart.Metadata `json:"meta"`
	}
)
