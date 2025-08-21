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
	Manifest       ocispec.Descriptor
	PluginData     []byte
	ProvenanceData []byte // Optional provenance data
	Ref            string
	PluginName     string
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

	// Find the required plugin tarball and optional provenance
	expectedTarball := pluginName + ".tgz"
	expectedProvenance := pluginName + ".tgz.prov"

	var pluginDescriptor *ocispec.Descriptor
	var provenanceDescriptor *ocispec.Descriptor

	// Look for layers with the expected titles/annotations
	for _, layer := range manifest.Layers {
		d := layer
		// Check for title annotation (preferred method)
		if title, exists := d.Annotations[ocispec.AnnotationTitle]; exists {
			switch title {
			case expectedTarball:
				pluginDescriptor = &d
			case expectedProvenance:
				provenanceDescriptor = &d
			}
		}
	}

	// Plugin tarball is required
	if pluginDescriptor == nil {
		return nil, fmt.Errorf("required layer %s not found in manifest", expectedTarball)
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
		result.ProvenanceData, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *provenanceDescriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve provenance data with digest %s: %w", provenanceDescriptor.Digest, err)
		}
	}

	fmt.Fprintf(c.out, "Pulled plugin: %s\n", result.Ref)
	fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	if result.ProvenanceData != nil {
		fmt.Fprintf(c.out, "Provenance: %s\n", expectedProvenance)
	}

	if strings.Contains(result.Ref, "_") {
		fmt.Fprintf(c.out, "%s contains an underscore.\n", result.Ref)
		fmt.Fprint(c.out, registryUnderscoreMessage+"\n")
	}

	return result, nil
}

// Plugin pull operation types and options
type (
	pluginPullOperation struct {
		pluginName string
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
