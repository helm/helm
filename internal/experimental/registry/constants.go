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

const (
	// OCIScheme is the URL scheme for OCI-based requests
	OCIScheme = "oci"

	// CredentialsFileBasename is the filename for auth credentials file
	CredentialsFileBasename = "config.json"

	// ConfigMediaType is the reserved media type for the Helm chart manifest config
	ConfigMediaType = "application/vnd.cncf.helm.config.v1+json"

	// ChartLayerMediaType is the reserved media type for Helm chart package content
	ChartLayerMediaType = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

	// ProvLayerMediaType is the reserved media type for Helm chart provenance files
	ProvLayerMediaType = "application/vnd.cncf.helm.chart.provenance.v1.prov"
)
