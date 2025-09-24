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

package registry // import "helm.sh/helm/v4/pkg/registry"

import (
	"bytes"
	"strings"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var immutableOciAnnotations = []string{
	ocispec.AnnotationVersion,
	ocispec.AnnotationTitle,
}

// extractChartMeta is used to extract a chart metadata from a byte array
func extractChartMeta(chartData []byte) (*chart.Metadata, error) {
	ch, err := loader.LoadArchive(bytes.NewReader(chartData))
	if err != nil {
		return nil, err
	}
	return ch.Metadata, nil
}

// generateOCIAnnotations will generate OCI annotations to include within the OCI manifest
func generateOCIAnnotations(meta *chart.Metadata, creationTime string) map[string]string {

	// Get annotations from Chart attributes
	ociAnnotations := generateChartOCIAnnotations(meta, creationTime)

	// Copy Chart annotations
annotations:
	for chartAnnotationKey, chartAnnotationValue := range meta.Annotations {

		// Avoid overriding key properties
		for _, immutableOciKey := range immutableOciAnnotations {
			if immutableOciKey == chartAnnotationKey {
				continue annotations
			}
		}

		// Add chart annotation
		ociAnnotations[chartAnnotationKey] = chartAnnotationValue
	}

	return ociAnnotations
}

// generateChartOCIAnnotations will generate OCI annotations from the provided chart
func generateChartOCIAnnotations(meta *chart.Metadata, creationTime string) map[string]string {
	chartOCIAnnotations := map[string]string{}

	chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationDescription, meta.Description)
	chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationTitle, meta.Name)
	chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationVersion, meta.Version)
	chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationURL, meta.Home)

	if len(creationTime) == 0 {
		creationTime = time.Now().UTC().Format(time.RFC3339)
	}

	chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationCreated, creationTime)

	if len(meta.Sources) > 0 {
		chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationSource, meta.Sources[0])
	}

	if len(meta.Maintainers) > 0 {
		var maintainerSb strings.Builder

		for maintainerIdx, maintainer := range meta.Maintainers {

			if len(maintainer.Name) > 0 {
				maintainerSb.WriteString(maintainer.Name)
			}

			if len(maintainer.Email) > 0 {
				maintainerSb.WriteString(" (")
				maintainerSb.WriteString(maintainer.Email)
				maintainerSb.WriteString(")")
			}

			if maintainerIdx < len(meta.Maintainers)-1 {
				maintainerSb.WriteString(", ")
			}

		}

		chartOCIAnnotations = addToMap(chartOCIAnnotations, ocispec.AnnotationAuthors, maintainerSb.String())

	}

	return chartOCIAnnotations
}

// addToMap takes an existing map and adds an item if the value is not empty
func addToMap(inputMap map[string]string, newKey string, newValue string) map[string]string {

	// Add item to map if its
	if len(strings.TrimSpace(newValue)) > 0 {
		inputMap[newKey] = newValue
	}

	return inputMap
}
