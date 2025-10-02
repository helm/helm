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
	"fmt"
	"reflect"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"

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

	chart := &chart.Metadata{
		Name:    "oci",
		Version: "0.0.1",
	}

	result := generateOCIAnnotations(chart, nowTimeString)

	// Check that created annotation exists
	if _, ok := result[ocispec.AnnotationCreated]; !ok {
		t.Errorf("%s annotation not created", ocispec.AnnotationCreated)
	}

	// Verify value of created artifact in RFC3339 format
	if _, err := time.Parse(time.RFC3339, result[ocispec.AnnotationCreated]); err != nil {
		t.Errorf("%s annotation with value '%s' not in RFC3339 format", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated])
	}

	// Verify default creation time set
	result = generateOCIAnnotations(chart, "")

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

func TestIsOCI(t *testing.T) {

	tests := []struct {
		name    string
		uri     string
		isValid bool
	}{
		{
			name:    "Valid URI",
			uri:     "oci://example.com/myregistry:1.2.3",
			isValid: true,
		},
		{
			name:    "Invalid URI prefix (boundary test 1)",
			uri:     "noci://example.com/myregistry:1.2.3",
			isValid: false,
		},
		{
			name:    "Invalid URI prefix (boundary test 2)",
			uri:     "ocin://example.com/myregistry:1.2.3",
			isValid: false,
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.isValid, IsOCI(tt.uri), tt.name)
	}
}

func TestContainsTag(t *testing.T) {

	tagList := []string{"1.0.0", "1.0.1", "2.0.0", "2.1.0"}

	tests := []struct {
		name    string
		tag     string
		present bool
	}{
		{
			name:    "tag present",
			tag:     "1.0.1",
			present: true,
		},
		{
			name:    "tag not present",
			tag:     "1.0.2",
			present: false,
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.present, ContainsTag(tagList, tt.tag), tt.name)
	}
}

func TestGetTagMatchingVersionOrConstraint(t *testing.T) {

	// GetTagMatchingVersionOrConstraint expects the tag list to be sorted highest to lowest version
	tagList := []string{
		"10.0.1",
		"9.0.1",
		"3.1.0",
		"3.0.0",
		"2.0.11",
		"2.0.9",
		"2.0.0",
		"1.10.0",
		"1.9.1",
		"1.9.0",
		"1.0.0",
		"0.3.0",
		"0.2.2",
		"0.2.1",
		"0.2.0",
		"0.1.0",
		"0.0.3",
		"0.0.2",
		"0.0.1",
	}

	tests := []struct {
		name              string
		versionConstraint string
		expectErr         bool
		expectVersion     string
	}{
		{
			name:              "Explicit version",
			versionConstraint: "1.10.0",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "Implicit wildcard default from empty string",
			versionConstraint: "",
			expectErr:         false,
			expectVersion:     "10.0.1",
		},
		{
			name:              "No versions within wildcard constraint",
			versionConstraint: "50.*",
			expectErr:         true,
			expectVersion:     "",
		},
		{
			name:              "Invalid version constraint",
			versionConstraint: "<>!=20",
			expectErr:         true,
			expectVersion:     "",
		},
		{
			name:              "Explicit wildcard",
			versionConstraint: "*",
			expectErr:         false,
			expectVersion:     "10.0.1",
		},
		{
			name:              "Major version wildcard selection",
			versionConstraint: "2.*",
			expectErr:         false,
			expectVersion:     "2.0.11",
		},
		{
			name:              "Minor version wildcard selection",
			versionConstraint: "3.1.*",
			expectErr:         false,
			expectVersion:     "3.1.0",
		},
		{
			name:              "~ major version",
			versionConstraint: "~1",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "~ major version plus x",
			versionConstraint: "~1.x",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "~ specific version",
			versionConstraint: "~1.9.0",
			expectErr:         false,
			expectVersion:     "1.9.1",
		},
		{
			name:              "~ minor version",
			versionConstraint: "~1.9",
			expectErr:         false,
			expectVersion:     "1.9.1",
		},
		{
			name:              "~ minor version plus x",
			versionConstraint: "~1.9.x",
			expectErr:         false,
			expectVersion:     "1.9.1",
		},
		{
			name:              "^ specific version",
			versionConstraint: "^1.9.0",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "^ minor version version >=1",
			versionConstraint: "^1.9",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "^ minor version plus x >=1",
			versionConstraint: "^1.9.x",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "^ major version plus x >=1",
			versionConstraint: "^1.x",
			expectErr:         false,
			expectVersion:     "1.10.0",
		},
		{
			name:              "^ full version <1",
			versionConstraint: "^0.2.1",
			expectErr:         false,
			expectVersion:     "0.2.2",
		},
		{
			name:              "^ minor version <1",
			versionConstraint: "^0.2",
			expectErr:         false,
			expectVersion:     "0.2.2",
		},
		{
			name:              "^ patch version < 0.1",
			versionConstraint: "^0.0.2",
			expectErr:         false,
			expectVersion:     "0.0.2",
		},
		{
			name:              "^ with 0.0",
			versionConstraint: "^0.0",
			expectErr:         false,
			expectVersion:     "0.0.3",
		},
		{
			name:              "^ with 0",
			versionConstraint: "^0",
			expectErr:         false,
			expectVersion:     "0.3.0",
		},
		{
			name:              "= operator",
			versionConstraint: "=1.9.0",
			expectErr:         false,
			expectVersion:     "1.9.0",
		},
		{
			name:              "!= operator",
			versionConstraint: "!=1.9.0",
			expectErr:         false,
			expectVersion:     "10.0.1",
		},
		{
			name:              "> operator",
			versionConstraint: ">1.9.0",
			expectErr:         false,
			expectVersion:     "10.0.1",
		},
		{
			name:              "< operator",
			versionConstraint: "<1.9.0",
			expectErr:         false,
			expectVersion:     "1.0.0",
		},
		{
			name:              ">= operator",
			versionConstraint: ">=1.9.0",
			expectErr:         false,
			expectVersion:     "10.0.1",
		},
		{
			name:              "<= operator",
			versionConstraint: "<=1.9.0",
			expectErr:         false,
			expectVersion:     "1.9.0",
		},
		{
			name:              "gte and less than combination",
			versionConstraint: ">=1.9.0, <1.10.0",
			expectErr:         false,
			expectVersion:     "1.9.1",
		},
		{
			name:              "gte and lte with a negation combination",
			versionConstraint: ">=1.9.0, <=1.10.0, !=1.10.0",
			expectErr:         false,
			expectVersion:     "1.9.1",
		},
		{
			name:              "multiple ranges separated by ||",
			versionConstraint: ">=1.1.0, <=1.10.0 || >=2.0.0, <3.0.0",
			expectErr:         false,
			expectVersion:     "2.0.11",
		},
	}

	for _, tt := range tests {
		version, err := GetTagMatchingVersionOrConstraint(tagList, tt.versionConstraint)
		if tt.expectErr {
			assert.Error(t, err, fmt.Sprintf("Should produce an error (%s)", tt.name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("Error should be nil (%s)", tt.name))
		}
		assert.Equal(t, tt.expectVersion, version, fmt.Sprintf("The expected matching version is %s (%s)", tt.expectVersion, tt.name))
	}
}
