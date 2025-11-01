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

package registry

import (
	"encoding/json"
	"fmt"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Plugin-specific constants
const (
	// PluginArtifactType is the artifact type for Helm plugins
	PluginArtifactType = "application/vnd.helm.plugin.v1+json"
)

// PluginPullOptions configures a plugin pull operation
type PluginPullOptions struct {
	// PluginName specifies the expected plugin name for layer validation
	PluginName string
}

// PluginPullResult contains the result of a plugin pull operation
type PluginPullResult struct {
	Manifest   ocispec.Descriptor
	PluginData []byte
	Prov       struct {
		Data []byte
	}
	Ref        string
	PluginName string
}

// PullPlugin downloads a plugin from an OCI registry using artifact type
func (c *Client) PullPlugin(ref string, pluginName string, options ...PluginPullOption) (*PluginPullResult, error) {
	operation := &pluginPullOperation{
		pluginName: pluginName,
	}
	for _, option := range options {
		option(operation)
	}

	// Use generic client for the pull operation with artifact type filtering
	genericClient := c.Generic()
	genericResult, err := genericClient.PullGeneric(ref, GenericPullOptions{
		// Allow manifests and all layer types - we'll validate artifact type after download
		AllowedMediaTypes: []string{
			ocispec.MediaTypeImageManifest,
			"application/vnd.oci.image.layer.v1.tar",
			"application/vnd.oci.image.layer.v1.tar+gzip",
		},
	})
	if err != nil {
		return nil, err
	}

	// Process the result with plugin-specific logic
	return c.processPluginPull(genericResult, operation.pluginName)
}

// processPluginPull handles plugin-specific processing of a generic pull result using artifact type
func (c *Client) processPluginPull(genericResult *GenericPullResult, pluginName string) (*PluginPullResult, error) {
	// First validate that this is actually a plugin artifact
	manifestData, err := c.Generic().GetDescriptorData(genericResult.MemoryStore, genericResult.Manifest)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve manifest: %w", err)
	}

	// Parse the manifest to check artifact type
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("unable to parse manifest: %w", err)
	}

	// Validate artifact type (for OCI v1.1+ manifests)
	if manifest.ArtifactType != "" && manifest.ArtifactType != PluginArtifactType {
		return nil, fmt.Errorf("expected artifact type %s, got %s", PluginArtifactType, manifest.ArtifactType)
	}

	// For backwards compatibility, also check config media type if no artifact type
	if manifest.ArtifactType == "" && manifest.Config.MediaType != PluginArtifactType {
		return nil, fmt.Errorf("expected config media type %s for legacy compatibility, got %s", PluginArtifactType, manifest.Config.MediaType)
	}

	// Find the plugin tarball and optional provenance using NAME-VERSION.tgz format
	var pluginDescriptor *ocispec.Descriptor
	var provenanceDescriptor *ocispec.Descriptor
	var foundProvenanceName string

	// Look for layers with the expected titles/annotations
	for _, layer := range manifest.Layers {
		d := layer
		// Check for title annotation
		if title, exists := d.Annotations[ocispec.AnnotationTitle]; exists {
			// Check if this looks like a plugin tarball: {pluginName}-{version}.tgz
			if pluginDescriptor == nil && strings.HasPrefix(title, pluginName+"-") && strings.HasSuffix(title, ".tgz") {
				pluginDescriptor = &d
			}
			// Check if this looks like a plugin provenance: {pluginName}-{version}.tgz.prov
			if provenanceDescriptor == nil && strings.HasPrefix(title, pluginName+"-") && strings.HasSuffix(title, ".tgz.prov") {
				provenanceDescriptor = &d
				foundProvenanceName = title
			}
		}
	}

	// Plugin tarball is required
	if pluginDescriptor == nil {
		return nil, fmt.Errorf("required layer matching pattern %s-VERSION.tgz not found in manifest", pluginName)
	}

	// Build plugin-specific result
	result := &PluginPullResult{
		Manifest:   genericResult.Manifest,
		Ref:        genericResult.Ref,
		PluginName: pluginName,
	}

	// Fetch plugin data using generic client
	genericClient := c.Generic()
	result.PluginData, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *pluginDescriptor)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve plugin data with digest %s: %w", pluginDescriptor.Digest, err)
	}

	// Fetch provenance data if available
	if provenanceDescriptor != nil {
		result.Prov.Data, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *provenanceDescriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve provenance data with digest %s: %w", provenanceDescriptor.Digest, err)
		}
	}

	_, _ = fmt.Fprintf(c.out, "Pulled plugin: %s\n", result.Ref)
	_, _ = fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	if result.Prov.Data != nil {
		_, _ = fmt.Fprintf(c.out, "Provenance: %s\n", foundProvenanceName)
	}

	if strings.Contains(result.Ref, "_") {
		_, _ = fmt.Fprintf(c.out, "%s contains an underscore.\n", result.Ref)
		_, _ = fmt.Fprint(c.out, registryUnderscoreMessage+"\n")
	}

	return result, nil
}

// Plugin pull operation types and options
type (
	pluginPullOperation struct {
		pluginName string
		withProv   bool
	}

	// PluginPullOption allows customizing plugin pull operations
	PluginPullOption func(*pluginPullOperation)
)

// PluginPullOptWithPluginName sets the plugin name for validation
func PluginPullOptWithPluginName(name string) PluginPullOption {
	return func(operation *pluginPullOperation) {
		operation.pluginName = name
	}
}

// GetPluginName extracts the plugin name from an OCI reference using proper reference parsing
func GetPluginName(source string) (string, error) {
	ref, err := newReference(source)
	if err != nil {
		return "", fmt.Errorf("invalid OCI reference: %w", err)
	}

	// Extract plugin name from the repository path
	// e.g., "ghcr.io/user/plugin-name:v1.0.0" -> Repository: "user/plugin-name"
	repository := ref.Repository
	if repository == "" {
		return "", fmt.Errorf("invalid OCI reference: missing repository")
	}

	// Get the last part of the repository path as the plugin name
	parts := strings.Split(repository, "/")
	pluginName := parts[len(parts)-1]

	if pluginName == "" {
		return "", fmt.Errorf("invalid OCI reference: cannot determine plugin name from repository %s", repository)
	}

	return pluginName, nil
}

// PullPluginOptWithProv configures the pull to fetch provenance data
func PullPluginOptWithProv(withProv bool) PluginPullOption {
	return func(operation *pluginPullOperation) {
		operation.withProv = withProv
	}
}
