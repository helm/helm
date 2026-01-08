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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// GenericClient provides low-level OCI operations without artifact-specific assumptions
type GenericClient struct {
	debug              bool
	enableCache        bool
	credentialsFile    string
	username           string
	password           string
	out                io.Writer
	authorizer         *auth.Client
	registryAuthorizer RemoteClient
	credentialsStore   credentials.Store
	httpClient         *http.Client
	plainHTTP          bool
}

// GenericPullOptions configures a generic pull operation
type GenericPullOptions struct {
	// MediaTypes to include in the pull (empty means all)
	AllowedMediaTypes []string
	// Skip descriptors with these media types
	SkipMediaTypes []string
	// Custom PreCopy function for filtering
	PreCopy func(context.Context, ocispec.Descriptor) error
	// ArtifactType to select from OCI Image Index (empty means no filtering).
	// When pulling from an Image Index containing multiple manifests,
	// this field is used to select the manifest with matching artifactType.
	ArtifactType string
}

// GenericPullResult contains the result of a generic pull operation
type GenericPullResult struct {
	Manifest    ocispec.Descriptor
	Descriptors []ocispec.Descriptor
	MemoryStore *memory.Store
	Ref         string
}

// NewGenericClient creates a new generic OCI client from an existing Client
func NewGenericClient(client *Client) *GenericClient {
	return &GenericClient{
		debug:              client.debug,
		enableCache:        client.enableCache,
		credentialsFile:    client.credentialsFile,
		username:           client.username,
		password:           client.password,
		out:                client.out,
		authorizer:         client.authorizer,
		registryAuthorizer: client.registryAuthorizer,
		credentialsStore:   client.credentialsStore,
		httpClient:         client.httpClient,
		plainHTTP:          client.plainHTTP,
	}
}

// resolveFromIndex selects a manifest from an OCI Image Index by artifactType.
// It returns the descriptor of the matching manifest, or an error if no match is found.
// If no manifests have artifactType set, it falls back to checking config.mediaType
// of each manifest to find one that matches the expected artifact type.
func (c *GenericClient) resolveFromIndex(ctx context.Context, repo *remote.Repository, indexDesc ocispec.Descriptor, artifactType string) (ocispec.Descriptor, error) {
	// Fetch the index manifest
	indexData, err := content.FetchAll(ctx, repo, indexDesc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to fetch image index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexData, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to parse image index: %w", err)
	}

	// First pass: look for explicit artifactType match
	var availableTypes []string
	var candidatesWithoutArtifactType []ocispec.Descriptor
	for _, manifest := range index.Manifests {
		if manifest.ArtifactType == artifactType {
			return manifest, nil
		}
		if manifest.ArtifactType != "" {
			availableTypes = append(availableTypes, manifest.ArtifactType)
		} else {
			// Collect manifests without artifactType for fallback check
			candidatesWithoutArtifactType = append(candidatesWithoutArtifactType, manifest)
		}
	}

	// Second pass: if no artifactType matches found, check config.mediaType of each manifest
	// This handles the case where artifacts are published without explicit artifactType
	for _, candidate := range candidatesWithoutArtifactType {
		// Skip manifests with platform (likely container images)
		if candidate.Platform != nil {
			continue
		}

		// Fetch the manifest to check its config.mediaType
		manifestData, err := content.FetchAll(ctx, repo, candidate)
		if err != nil {
			continue // Skip manifests we can't fetch
		}

		var manifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			continue // Skip malformed manifests
		}

		// Check if config.mediaType matches our expected artifact type
		if manifest.Config.MediaType == artifactType {
			return candidate, nil
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf(
		"no manifest with artifactType %q found in image index; available types: %v",
		artifactType, availableTypes)
}

// PullGeneric performs a generic OCI pull without artifact-specific assumptions
func (c *GenericClient) PullGeneric(ref string, options GenericPullOptions) (*GenericPullResult, error) {
	parsedRef, err := newReference(ref)
	if err != nil {
		return nil, err
	}

	memoryStore := memory.New()
	var descriptors []ocispec.Descriptor

	// Set up a repository with authentication and configuration
	repository, err := remote.NewRepository(parsedRef.String())
	if err != nil {
		return nil, err
	}
	repository.PlainHTTP = c.plainHTTP
	repository.Client = c.authorizer

	ctx := context.Background()

	// Resolve the reference to get the manifest descriptor
	// This allows us to detect Image Index and select the appropriate manifest
	pullRef := parsedRef.String()
	if options.ArtifactType != "" {
		// Try to resolve the reference to check if it's an Image Index.
		// If resolution fails, continue with normal pull - the error will
		// manifest during oras.Copy() if there's a real problem.
		resolvedDesc, err := repository.Resolve(ctx, pullRef)
		if err == nil && resolvedDesc.MediaType == ocispec.MediaTypeImageIndex {
			// Select the manifest with matching artifactType from the index
			selectedManifest, err := c.resolveFromIndex(ctx, repository, resolvedDesc, options.ArtifactType)
			if err != nil {
				return nil, err
			}
			// Use the selected manifest's digest for pulling
			pullRef = selectedManifest.Digest.String()
		}
		// If Resolve() failed or it's not an Image Index, continue with original pullRef
	}

	// Prepare allowed media types for filtering
	var allowedMediaTypes []string
	if len(options.AllowedMediaTypes) > 0 {
		allowedMediaTypes = make([]string, len(options.AllowedMediaTypes))
		copy(allowedMediaTypes, options.AllowedMediaTypes)
		sort.Strings(allowedMediaTypes)
	}

	var mu sync.Mutex
	manifest, err := oras.Copy(ctx, repository, pullRef, memoryStore, "", oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			PreCopy: func(ctx context.Context, desc ocispec.Descriptor) error {
				// Apply a custom PreCopy function if provided
				if options.PreCopy != nil {
					if err := options.PreCopy(ctx, desc); err != nil {
						return err
					}
				}

				mediaType := desc.MediaType

				// Skip media types if specified
				if slices.Contains(options.SkipMediaTypes, mediaType) {
					return oras.SkipNode
				}

				// Filter by allowed media types if specified
				if len(allowedMediaTypes) > 0 {
					if i := sort.SearchStrings(allowedMediaTypes, mediaType); i >= len(allowedMediaTypes) || allowedMediaTypes[i] != mediaType {
						return oras.SkipNode
					}
				}

				mu.Lock()
				descriptors = append(descriptors, desc)
				mu.Unlock()
				return nil
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &GenericPullResult{
		Manifest:    manifest,
		Descriptors: descriptors,
		MemoryStore: memoryStore,
		Ref:         parsedRef.String(),
	}, nil
}

// GetDescriptorData retrieves the data for a specific descriptor
func (c *GenericClient) GetDescriptorData(store *memory.Store, desc ocispec.Descriptor) ([]byte, error) {
	return content.FetchAll(context.Background(), store, desc)
}
