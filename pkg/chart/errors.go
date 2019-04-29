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

import "github.com/pkg/errors"

// ErrMissingMetadata indicates that Chart.yaml is missing
var ErrMissingMetadata = errors.New("validation: chart.metadata (Chart.yaml) is required")

// ErrMissingAPIVersion indicates that chart apiVersion is missing in Chart.yaml
var ErrMissingAPIVersion = errors.New("validation: chart.metadata.apiVersion is required in Chart.yaml")

// ErrMissingName indicates that chart name is missing in Chart.yaml
var ErrMissingName = errors.New("validation: chart.metadata.name is required in Chart.yaml")

// ErrMissingVersion indicates that chart version is missing in Chart.yaml
var ErrMissingVersion = errors.New("validation: chart.metadata.version is required in Chart.yaml")

// ErrInvalidType indicates that chart type is invalid in Chart.yaml
var ErrInvalidType = errors.New("validation: chart.metadata.type must be 'application' or 'library' in Chart.yaml")
