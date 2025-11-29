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
	"reflect"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestGenerateOCIChartAnnotations(t *testing.T) {

	nowString := time.Now().Format(time.RFC3339)

	tests := []struct {
		name   string
		chart  *chart.Metadata
		expect map[string]string
	}{
		{
			"Baseline chart",
			&chart.Metadata{
				Name:    "oci",
				Version: "0.0.1",
			},
			map[string]string{
				"org.opencontainers.image.title":   "oci",
				"org.opencontainers.image.version": "0.0.1",
				"org.opencontainers.image.created": nowString,
			},
		},
		{
			"Simple chart values",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Home:        "https://helm.sh",
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.created":     nowString,
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.url":         "https://helm.sh",
			},
		},
		{
			"Maintainer without email",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Home:        "https://helm.sh",
				Maintainers: []*chart.Maintainer{
					{
						Name: "John Snow",
					},
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.created":     nowString,
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.url":         "https://helm.sh",
				"org.opencontainers.image.authors":     "John Snow",
			},
		},
		{
			"Maintainer with email",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Home:        "https://helm.sh",
				Maintainers: []*chart.Maintainer{
					{Name: "John Snow", Email: "john@winterfell.com"},
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.created":     nowString,
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.url":         "https://helm.sh",
				"org.opencontainers.image.authors":     "John Snow (john@winterfell.com)",
			},
		},
		{
			"Multiple Maintainers",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Home:        "https://helm.sh",
				Maintainers: []*chart.Maintainer{
					{Name: "John Snow", Email: "john@winterfell.com"},
					{Name: "Jane Snow"},
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.created":     nowString,
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.url":         "https://helm.sh",
				"org.opencontainers.image.authors":     "John Snow (john@winterfell.com), Jane Snow",
			},
		},
		{
			"Chart with Sources",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Sources: []string{
					"https://github.com/helm/helm",
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.created":     nowString,
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.source":      "https://github.com/helm/helm",
			},
		},
	}

	for _, tt := range tests {

		result := generateChartOCIAnnotations(tt.chart, nowString)

		if !reflect.DeepEqual(tt.expect, result) {
			t.Errorf("%s: expected map %v, got %v", tt.name, tt.expect, result)
		}

	}
}

func TestGenerateOCIAnnotations(t *testing.T) {

	nowString := time.Now().Format(time.RFC3339)

	tests := []struct {
		name   string
		chart  *chart.Metadata
		expect map[string]string
	}{
		{
			"Baseline chart",
			&chart.Metadata{
				Name:    "oci",
				Version: "0.0.1",
			},
			map[string]string{
				"org.opencontainers.image.title":   "oci",
				"org.opencontainers.image.version": "0.0.1",
				"org.opencontainers.image.created": nowString,
			},
		},
		{
			"Simple chart values with custom Annotations",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Annotations: map[string]string{
					"extrakey":   "extravlue",
					"anotherkey": "anothervalue",
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.created":     nowString,
				"extrakey":                             "extravlue",
				"anotherkey":                           "anothervalue",
			},
		},
		{
			"Verify Chart Name and Version cannot be overridden from annotations",
			&chart.Metadata{
				Name:        "oci",
				Version:     "0.0.1",
				Description: "OCI Helm Chart",
				Annotations: map[string]string{
					"org.opencontainers.image.title":   "badchartname",
					"org.opencontainers.image.version": "1.0.0",
					"extrakey":                         "extravlue",
				},
			},
			map[string]string{
				"org.opencontainers.image.title":       "oci",
				"org.opencontainers.image.version":     "0.0.1",
				"org.opencontainers.image.description": "OCI Helm Chart",
				"org.opencontainers.image.created":     nowString,
				"extrakey":                             "extravlue",
			},
		},
	}

	for _, tt := range tests {

		result := generateOCIAnnotations(tt.chart, nowString)

		if !reflect.DeepEqual(tt.expect, result) {
			t.Errorf("%s: expected map %v, got %v", tt.name, tt.expect, result)
		}

	}
}

func TestGenerateOCICreatedAnnotations(t *testing.T) {

	nowTime := time.Now()
	nowTimeString := nowTime.Format(time.RFC3339)

	testChart := &chart.Metadata{
		Name:    "oci",
		Version: "0.0.1",
	}

	result := generateOCIAnnotations(testChart, nowTimeString)

	// Check that created annotation exists
	if _, ok := result[ocispec.AnnotationCreated]; !ok {
		t.Errorf("%s annotation not created", ocispec.AnnotationCreated)
	}

	// Verify value of created artifact in RFC3339 format
	if _, err := time.Parse(time.RFC3339, result[ocispec.AnnotationCreated]); err != nil {
		t.Errorf("%s annotation with value '%s' not in RFC3339 format", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated])
	}

	// Verify default creation time set
	result = generateOCIAnnotations(testChart, "")

	// Check that created annotation exists
	if _, ok := result[ocispec.AnnotationCreated]; !ok {
		t.Errorf("%s annotation not created", ocispec.AnnotationCreated)
	}

	if createdTimeAnnotation, err := time.Parse(time.RFC3339, result[ocispec.AnnotationCreated]); err != nil {
		t.Errorf("%s annotation with value '%s' not in RFC3339 format", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated])

		// Verify creation annotation after time test began
		if !nowTime.Before(createdTimeAnnotation) {
			t.Errorf("%s annotation with value '%s' not configured properly. Annotation value is not after %s", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated], nowTimeString)
		}

	}

}

func TestEscapeNonASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ASCII only - no change",
			input:    "John Smith",
			expected: "John Smith",
		},
		{
			name:     "German umlaut",
			input:    "Jan-Otto Kröpke",
			expected: "Jan-Otto Kr\\u00f6pke",
		},
		{
			name:     "Multiple umlauts",
			input:    "Müller Schröder",
			expected: "M\\u00fcller Schr\\u00f6der",
		},
		{
			name:     "French accents",
			input:    "François Müller",
			expected: "Fran\\u00e7ois M\\u00fcller",
		},
		{
			name:     "Spanish tilde",
			input:    "José Muñoz",
			expected: "Jos\\u00e9 Mu\\u00f1oz",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Numbers and punctuation",
			input:    "v1.2.3-beta+build.123",
			expected: "v1.2.3-beta+build.123",
		},
		{
			name:     "Email with umlaut in name",
			input:    "kröpke@example.com",
			expected: "kr\\u00f6pke@example.com",
		},
		{
			name:     "Nordic characters",
			input:    "Øystein Ålander",
			expected: "\\u00d8ystein \\u00c5lander",
		},
		{
			name:     "Mixed ASCII and non-ASCII",
			input:    "Abc Déf Ghï",
			expected: "Abc D\\u00e9f Gh\\u00ef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeNonASCII(tt.input)
			if result != tt.expected {
				t.Errorf("escapeNonASCII(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateChartOCIAnnotations_WithNonASCII(t *testing.T) {
	meta := &chart.Metadata{
		Name:        "test-chart",
		Description: "A Helm chart for Kröpke",
		Version:     "1.0.0",
		Home:        "https://example.com",
		Sources:     []string{"https://github.com/example/test"},
		Maintainers: []*chart.Maintainer{
			{
				Name:  "Jan-Otto Kröpke",
				Email: "github@jkroepke.de",
			},
			{
				Name:  "François Müller",
				Email: "francois@example.com",
			},
		},
	}

	annotations := generateChartOCIAnnotations(meta, "2025-01-01T00:00:00Z")

	// Check description is escaped
	expectedDesc := "A Helm chart for Kr\\u00f6pke"
	if annotations["org.opencontainers.image.description"] != expectedDesc {
		t.Errorf("Description = %q, want %q",
			annotations["org.opencontainers.image.description"], expectedDesc)
	}

	// Check authors are escaped
	expectedAuthors := "Jan-Otto Kr\\u00f6pke (github@jkroepke.de), Fran\\u00e7ois M\\u00fcller (francois@example.com)"
	if annotations["org.opencontainers.image.authors"] != expectedAuthors {
		t.Errorf("Authors = %q, want %q",
			annotations["org.opencontainers.image.authors"], expectedAuthors)
	}

	// Check title (ASCII only, should be unchanged)
	if annotations["org.opencontainers.image.title"] != "test-chart" {
		t.Errorf("Title = %q, want %q",
			annotations["org.opencontainers.image.title"], "test-chart")
	}
}

func TestGenerateChartOCIAnnotations_ASCIIOnly(t *testing.T) {
	meta := &chart.Metadata{
		Name:        "test-chart",
		Description: "A simple Helm chart",
		Version:     "1.0.0",
		Maintainers: []*chart.Maintainer{
			{
				Name:  "John Smith",
				Email: "john@example.com",
			},
		},
	}

	annotations := generateChartOCIAnnotations(meta, "2025-01-01T00:00:00Z")

	// ASCII-only values should be unchanged
	if annotations["org.opencontainers.image.description"] != "A simple Helm chart" {
		t.Errorf("Description should be unchanged for ASCII-only input")
	}

	if annotations["org.opencontainers.image.authors"] != "John Smith (john@example.com)" {
		t.Errorf("Authors should be unchanged for ASCII-only input")
	}
}