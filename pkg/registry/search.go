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
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

type (
	// SearchOption allows specifying various settings on search
	SearchOption func(*searchOperation)

	searchOperation struct {
		version  string
		versions bool
	}

	// SearchResult is the result returned upon successful search.
	SearchResult struct {
		Charts []*SearchResultChart `json:"charts"`
	}

	// SearchResultChart represents a single chart version found in a registry.
	SearchResultChart struct {
		// Reference is the full OCI reference (e.g., oci://registry/repo/chart)
		Reference string `json:"reference"`
		// Chart contains the chart metadata extracted from the OCI config layer
		Chart *chart.Metadata `json:"chart"`
	}
)

// SearchOptVersion sets the version constraint for search
func SearchOptVersion(version string) SearchOption {
	return func(operation *searchOperation) {
		operation.version = version
	}
}

// SearchOptVersions sets whether to return all matching versions
func SearchOptVersions(versions bool) SearchOption {
	return func(operation *searchOperation) {
		operation.versions = versions
	}
}

// Search queries an OCI registry for chart versions matching the given reference.
// It lists all tags for the repository, filters by semver constraint, and fetches
// chart metadata from each matching tag's config layer.
func (c *Client) Search(ref string, options ...SearchOption) (*SearchResult, error) {
	searchResult := &SearchResult{
		Charts: []*SearchResultChart{},
	}

	operation := &searchOperation{}
	for _, option := range options {
		option(operation)
	}

	// List all tags for the repository
	tags, err := c.Tags(ref)
	if err != nil {
		// If the registry doesn't support tag listing, return empty results
		if strings.Contains(err.Error(), "unexpected status code") {
			slog.Debug("registry does not support tag listing", slog.String("ref", ref), slog.Any("error", err))
			return searchResult, nil
		}
		return searchResult, err
	}

	// Filter tags by version constraint
	var matchingTags []string
	for _, tag := range tags {
		match, err := GetTagMatchingVersionOrConstraint([]string{tag}, operation.version)
		if err == nil {
			matchingTags = append(matchingTags, match)
		}
	}

	parsedRef, err := newReference(ref)
	if err != nil {
		return searchResult, err
	}

	ociRef := fmt.Sprintf("%s://%s/%s", OCIScheme, parsedRef.Registry, parsedRef.Repository)

	// Fetch chart metadata for each matching tag
	for _, tag := range matchingTags {
		tagRef := fmt.Sprintf("%s/%s:%s", parsedRef.Registry, parsedRef.Repository, strings.ReplaceAll(tag, "+", "_"))

		meta, err := c.fetchChartMetadata(tagRef)
		if err != nil {
			slog.Debug("failed to fetch chart metadata", slog.String("ref", tagRef), slog.Any("error", err))
			continue
		}

		searchResult.Charts = append(searchResult.Charts, &SearchResultChart{
			Reference: ociRef,
			Chart:     meta,
		})

		// If not listing all versions, return only the latest (first match, since tags are sorted descending)
		if !operation.versions {
			break
		}
	}

	return searchResult, nil
}

// fetchChartMetadata pulls only the config layer from an OCI manifest to extract chart metadata.
// This avoids downloading the full chart tarball.
func (c *Client) fetchChartMetadata(ref string) (*chart.Metadata, error) {
	genericClient := c.Generic()

	// Only fetch the manifest and config layer, skip the chart tarball
	genericResult, err := genericClient.PullGeneric(ref, GenericPullOptions{
		AllowedMediaTypes: []string{
			ocispec.MediaTypeImageManifest,
			ocispec.MediaTypeImageIndex,
			ConfigMediaType,
		},
	})
	if err != nil {
		return nil, err
	}

	// Find the config descriptor
	var configDescriptor *ocispec.Descriptor
	for _, desc := range genericResult.Descriptors {
		d := desc
		if d.MediaType == ConfigMediaType {
			configDescriptor = &d
			break
		}
	}

	if configDescriptor == nil {
		return nil, fmt.Errorf("could not find config layer with mediatype %s", ConfigMediaType)
	}

	// Fetch and parse the config data
	configData, err := genericClient.GetDescriptorData(genericResult.MemoryStore, *configDescriptor)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve config blob: %w", err)
	}

	var meta chart.Metadata
	if err := json.Unmarshal(configData, &meta); err != nil {
		return nil, fmt.Errorf("unable to parse chart metadata: %w", err)
	}

	return &meta, nil
}
