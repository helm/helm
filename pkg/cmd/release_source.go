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

package cmd

import (
	"net/url"

	v3chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

// setReleaseSource records, on the chart's metadata, the location the chart was
// installed or upgraded from so it can be surfaced by 'helm list --show-source'.
// A repository URL (when supplied) is preferred over the local chart reference,
// and any embedded credentials are stripped before the value is persisted.
//
// It accepts the loader's chart.Charter (which may be either a v2 or v3 chart)
// and is a no-op for any unrecognized chart type, so callers never need to
// reason about the concrete chart version.
func setReleaseSource(chrt chart.Charter, chartRef, repoURL string) {
	source := chartRef
	if repoURL != "" {
		source = repoURL
	}
	source = sanitizeChartSource(source)

	switch c := chrt.(type) {
	case *v2chart.Chart:
		if c.Metadata == nil {
			c.Metadata = &v2chart.Metadata{}
		}
		if c.Metadata.Annotations == nil {
			c.Metadata.Annotations = make(map[string]string)
		}
		c.Metadata.Annotations[action.ReleaseSourceAnnotation] = source
	case *v3chart.Chart:
		if c.Metadata == nil {
			c.Metadata = &v3chart.Metadata{}
		}
		if c.Metadata.Annotations == nil {
			c.Metadata.Annotations = make(map[string]string)
		}
		c.Metadata.Annotations[action.ReleaseSourceAnnotation] = source
	}
}

// sanitizeChartSource strips any user credentials and other potentially
// sensitive components (query string, fragment) from a chart source URL before
// it is persisted into the release record, so secrets embedded in a repo URL
// (e.g. https://user:pass@host) are not leaked into cluster metadata. Sources
// that are not URLs (local paths or chart references) are returned unchanged.
//
// Only sources with a host component (i.e. scheme://host/...) are treated as
// URLs. This deliberately excludes local filesystem paths, including Windows
// absolute paths such as C:\charts\mychart, which url.Parse would otherwise
// interpret as a "c" scheme and re-encode (e.g. c:%5Ccharts%5Cmychart).
func sanitizeChartSource(source string) string {
	u, err := url.Parse(source)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return source
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
