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
)

// chartSource resolves the location a chart was installed or upgraded from, for
// persistence under action.ReleaseSourceAnnotation. A repository URL (when
// supplied) is preferred over the local chart reference, and any embedded
// credentials are stripped before the value is returned. The result is handed
// to the install/upgrade action, which records it on the release after
// rendering so it is never exposed to templates via .Chart.Annotations.
func chartSource(chartRef, repoURL string) string {
	source := chartRef
	if repoURL != "" {
		source = repoURL
	}
	return sanitizeChartSource(source)
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
