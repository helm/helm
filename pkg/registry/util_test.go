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
	"github.com/stretchr/testify/assert"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	helmtime "helm.sh/helm/v4/pkg/time"
)

func TestGenerateOCIChartAnnotations(t *testing.T) {

	nowString := helmtime.Now().Format(time.RFC3339)

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

	nowString := helmtime.Now().Format(time.RFC3339)

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

	nowTime := helmtime.Now()
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
	if _, err := helmtime.Parse(time.RFC3339, result[ocispec.AnnotationCreated]); err != nil {
		t.Errorf("%s annotation with value '%s' not in RFC3339 format", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated])
	}

	// Verify default creation time set
	result = generateOCIAnnotations(chart, "")

	// Check that created annotation exists
	if _, ok := result[ocispec.AnnotationCreated]; !ok {
		t.Errorf("%s annotation not created", ocispec.AnnotationCreated)
	}

	if createdTimeAnnotation, err := helmtime.Parse(time.RFC3339, result[ocispec.AnnotationCreated]); err != nil {
		t.Errorf("%s annotation with value '%s' not in RFC3339 format", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated])

		// Verify creation annotation after time test began
		if !nowTime.Before(createdTimeAnnotation) {
			t.Errorf("%s annotation with value '%s' not configured properly. Annotation value is not after %s", ocispec.AnnotationCreated, result[ocispec.AnnotationCreated], nowTimeString)
		}

	}

}

func TestIsOCI(t *testing.T) {

	assert.True(t, IsOCI("oci://example.com/myregistry:1.2.3"), "OCI URL marked as invalid")
	assert.False(t, IsOCI("noci://example.com/myregistry:1.2.3"), "Invalid OCI URL marked as valid")
	assert.False(t, IsOCI("ocin://example.com/myregistry:1.2.3"), "Invalid OCI URL marked as valid")
}

func TestContainsTag(t *testing.T) {

	tagList := []string{"1.0.0", "1.0.1", "2.0.0", "2.1.0"}
	assert.True(t, ContainsTag(tagList, "1.0.1"), "The tag 1.0.1 is in the list")
	assert.False(t, ContainsTag(tagList, "1.0.2"), "the tag 1.0.2 is not in the list")
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

	// Explicit verion
	version, err := GetTagMatchingVersionOrConstraint(tagList, "1.10.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	// Implicit wildcard default from empty string
	version, err = GetTagMatchingVersionOrConstraint(tagList, "")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "10.0.1", version, "The expected matching version is 10.0.1")

	// No versions within wildcard constraint
	version, err = GetTagMatchingVersionOrConstraint(tagList, "50.*")
	assert.Error(t, err, "An invalid version constraint (no valid version found) should produce an error")
	assert.Equal(t, "", version, "An invalid version constraint (no valid version found) should return an empty string as the version")

	// Invalid version constraint
	version, err = GetTagMatchingVersionOrConstraint(tagList, "<>!=20")
	assert.Error(t, err, "An invalid version constraint (non-parsable constraint) should produce an error")
	assert.Equal(t, "", version, "An invalid version constraint (non-parseable constraint) should return an empty string as the version")

	// Explicit wildcard
	version, err = GetTagMatchingVersionOrConstraint(tagList, "*")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "10.0.1", version, "The expected matching version is 10.0.1")

	// Major version wildcard selection
	version, err = GetTagMatchingVersionOrConstraint(tagList, "2.*")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "2.0.11", version, "The expected matching version is 2.0.11")

	// Minor version wildcard selection
	version, err = GetTagMatchingVersionOrConstraint(tagList, "3.1.*")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "3.1.0", version, "The expected matching version is 3.1.0")

	// ~ major
	version, err = GetTagMatchingVersionOrConstraint(tagList, "~1")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "~1.x")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	// ~ specific version
	version, err = GetTagMatchingVersionOrConstraint(tagList, "~1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.1", version, "The expected matching version is 1.9.1")

	// ~ minor version
	version, err = GetTagMatchingVersionOrConstraint(tagList, "~1.9")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.1", version, "The expected matching version is 1.9.1")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "~1.9.x")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.1", version, "The expected matching version is 1.9.1")

	// ^ specific version
	version, err = GetTagMatchingVersionOrConstraint(tagList, "^1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	// ^ major version >= 1
	version, err = GetTagMatchingVersionOrConstraint(tagList, "^1.9.x")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "^1.9")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "^1.x")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.10.0", version, "The expected matching version is 1.10.0")

	// ^ minor version < 1
	version, err = GetTagMatchingVersionOrConstraint(tagList, "^0.2.1")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "0.2.2", version, "The expected matching version is 0.2.2")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "^0.2")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "0.2.2", version, "The expected matching version is 0.2.2")

	// ^ patch version
	version, err = GetTagMatchingVersionOrConstraint(tagList, "^0.0.2")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "0.0.2", version, "The expected matching version is 0.0.2")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "^0.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "0.0.3", version, "The expected matching version is 0.0.3")

	version, err = GetTagMatchingVersionOrConstraint(tagList, "^0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "0.3.0", version, "The expected matching version is 0.3.0")

	// =
	version, err = GetTagMatchingVersionOrConstraint(tagList, "=1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.0", version, "The expected matching version is 1.9.0")

	// !=
	version, err = GetTagMatchingVersionOrConstraint(tagList, "!=1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "10.0.1", version, "The expected matching version is 10.0.1")

	// >
	version, err = GetTagMatchingVersionOrConstraint(tagList, ">1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "10.0.1", version, "The expected matching version is 10.0.1")

	// <
	version, err = GetTagMatchingVersionOrConstraint(tagList, "<1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.0.0", version, "The expected matching version is 1.0.0")

	// >=
	version, err = GetTagMatchingVersionOrConstraint(tagList, ">=1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "10.0.1", version, "The expected matching version is 10.0.1")

	// <=
	version, err = GetTagMatchingVersionOrConstraint(tagList, "<=1.9.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.0", version, "The expected matching version is 1.9.0")

	// , separation
	version, err = GetTagMatchingVersionOrConstraint(tagList, ">=1.9.0, <1.10.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.1", version, "The expected matching version is 1.9.1")

	version, err = GetTagMatchingVersionOrConstraint(tagList, ">=1.9.0, <=1.10.0, !=1.10.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "1.9.1", version, "The expected matching version is 1.9.1")

	// ||
	version, err = GetTagMatchingVersionOrConstraint(tagList, ">=1.1.0, <=1.10.0 || >=2.0.0, <3.0.0")
	assert.Nil(t, err, "Valid tag constraint query should not produce an error")
	assert.Equal(t, "2.0.11", version, "The expected matching version is 2.0.11")
}
