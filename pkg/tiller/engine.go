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

package tiller

import (
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chartutil"
)

// Engine represents a template engine that can render templates.
//
// For some engines, "rendering" includes both compiling and executing. (Other
// engines do not distinguish between phases.)
//
// The engine returns a map where the key is the named output entity (usually
// a file name) and the value is the rendered content of the template.
//
// An Engine must be capable of executing multiple concurrent requests, but
// without tainting one request's environment with data from another request.
type Engine interface {
	// Render renders a chart.
	//
	// It receives a chart, a config, and a map of overrides to the config.
	// Overrides are assumed to be passed from the system, not the user.
	Render(*chart.Chart, chartutil.Values) (map[string]string, error)
}
