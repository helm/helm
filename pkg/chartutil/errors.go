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

package chartutil

import (
	"fmt"
)

var (
	// ErrChartMetadataMissing is raised when a Chart.yaml is missing from a chart.
	ErrChartMetadataMissing = fmt.Errorf("chart metadata (%s) missing", ChartfileName)
	// ErrChartNameEmpty is raised when a name is not specified in a Chart.yaml.
	ErrChartNameEmpty = fmt.Errorf("invalid chart (%s): name must not be empty", ChartfileName)
	// ErrChartVersionEmpty is raised when a version is not specified in a Chart.yaml.
	ErrChartVersionEmpty = fmt.Errorf("invalid chart (%s): version must not be empty", ChartfileName)
)
