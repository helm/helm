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

package registry // import "helm.sh/helm/pkg/registry"

const (
	// HelmChartDefaultTag is the default tag used when storing a chart reference with no tag
	HelmChartDefaultTag = "latest"

	// HelmChartMetaMediaType is the reserved media type for Helm chart metadata
	HelmChartMetaMediaType = "application/vnd.cncf.helm.chart.meta.v1+json"

	// HelmChartContentMediaType is the reserved media type for Helm chart package content
	HelmChartContentMediaType = "application/vnd.cncf.helm.chart.content.v1+tar"

	// HelmChartMetaFileName is the reserved file name for Helm chart metadata
	HelmChartMetaFileName = "chart-meta.json"

	// HelmChartContentFileName is the reserved file name for Helm chart package content
	HelmChartContentFileName = "chart-content.tgz"

	// HelmChartNameAnnotation is the reserved annotation key for Helm chart name
	HelmChartNameAnnotation = "sh.helm.chart.name"

	// HelmChartVersionAnnotation is the reserved annotation key for Helm chart version
	HelmChartVersionAnnotation = "sh.helm.chart.version"
)

// KnownMediaTypes returns a list of layer mediaTypes that the Helm client knows about
func KnownMediaTypes() []string {
	return []string{
		HelmChartMetaMediaType,
		HelmChartContentMediaType,
	}
}
