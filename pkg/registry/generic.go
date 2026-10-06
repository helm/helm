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
	"strings"
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
	// Selectors are descriptor annotations used to disambiguate when more than
	// one manifest in an Image Index matches ArtifactType. A manifest matches
	// only if its descriptor annotations contain every selector key with the
	// same value. With a single matching manifest, selectors are not consulted.
	Selectors map[string]string
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
func (c *GenericClient) resolveFromIndex(ctx context.Context, repo *remote.Repository, indexDesc ocispec.Descriptor, artifactType string, selectors map[string]string) (ocispec.Descriptor, error) {
	// Fetch the index manifest
	indexData, err := content.FetchAll(ctx, repo, indexDesc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to fetch image index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexData, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to parse image index: %w", err)
	}

	// First pass: collect every manifest whose artifactType matches. A chart is
	// never platform-specific, so descriptors carrying a platform (container
	// images) are skipped outright.
	var candidates []ocispec.Descriptor
	var availableTypes []string
	var withoutArtifactType []ocispec.Descriptor
	for _, manifest := range index.Manifests {
		if manifest.Platform != nil {
			continue
		}
		switch {
		case manifest.ArtifactType == artifactType:
			candidates = append(candidates, manifest)
		case manifest.ArtifactType != "":
			availableTypes = append(availableTypes, manifest.ArtifactType)
		default:
			// No artifactType set: keep for the config.mediaType fallback below.
			withoutArtifactType = append(withoutArtifactType, manifest)
		}
	}

	// Second pass: legacy indexes whose descriptors carry no artifactType. Fetch
	// those manifests and match on config.mediaType instead. Cache each chart's
	// config descriptor so a later identity lookup need not refetch the manifest.
	configCache := map[string]ocispec.Descriptor{}
	if len(candidates) == 0 {
		for _, candidate := range withoutArtifactType {
			manifestData, err := content.FetchAll(ctx, repo, candidate)
			if err != nil {
				continue // Skip manifests we can't fetch
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(manifestData, &manifest); err != nil {
				continue // Skip malformed manifests
			}
			if manifest.Config.MediaType == artifactType {
				candidates = append(candidates, candidate)
				configCache[candidate.Digest.String()] = manifest.Config
			}
		}
	}

	switch len(candidates) {
	case 0:
		return ocispec.Descriptor{}, fmt.Errorf(
			"no manifest with artifactType %q found in image index; available types: %v",
			artifactType, availableTypes)
	case 1:
		return candidates[0], nil
	}

	// More than one chart in the index: disambiguate by the requested chart name
	// (required), then by the requested version as a tie-breaker. Returning the
	// first match here is what silently delivered the wrong chart, so an
	// unresolvable choice is reported rather than guessed.
	resolved := make([]chartCandidate, 0, len(candidates))
	for _, d := range candidates {
		name, version := c.chartIdentity(ctx, repo, d, configCache)
		resolved = append(resolved, chartCandidate{desc: d, name: name, version: version})
	}

	wantName := selectors[ocispec.AnnotationTitle]
	if wantName == "" {
		return ocispec.Descriptor{}, fmt.Errorf(
			"image index holds %d charts and no chart name was given to disambiguate; candidates: %s",
			len(resolved), describeCandidates(resolved))
	}

	var named []chartCandidate
	for _, cand := range resolved {
		if cand.name == wantName {
			named = append(named, cand)
		}
	}
	switch len(named) {
	case 0:
		return ocispec.Descriptor{}, fmt.Errorf(
			"image index holds %d charts and none is named %q; candidates: %s",
			len(resolved), wantName, describeCandidates(resolved))
	case 1:
		return named[0].desc, nil
	}

	// Several charts share the requested name: break the tie by version.
	if wantVersion := selectors[ocispec.AnnotationVersion]; wantVersion != "" {
		var versioned []chartCandidate
		for _, cand := range named {
			if cand.version == wantVersion {
				versioned = append(versioned, cand)
			}
		}
		if len(versioned) == 1 {
			return versioned[0].desc, nil
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf(
		"image index is ambiguous: %d charts named %q; specify a version; candidates: %s",
		len(named), wantName, describeCandidates(named))
}

// chartCandidate pairs an index descriptor with the chart identity used to
// disambiguate it.
type chartCandidate struct {
	desc    ocispec.Descriptor
	name    string
	version string
}

// chartIdentity returns a candidate's chart name and version, preferring the
// descriptor annotations and falling back to the chart config (Chart.yaml)
// referenced by the manifest when the annotations are absent (legacy indexes).
func (c *GenericClient) chartIdentity(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor, configCache map[string]ocispec.Descriptor) (name, version string) {
	name = desc.Annotations[ocispec.AnnotationTitle]
	version = desc.Annotations[ocispec.AnnotationVersion]
	if name != "" {
		return name, version
	}

	// Resolve the chart config descriptor, reusing the one captured during the
	// legacy pass when present so the manifest is not fetched a second time.
	config, ok := configCache[desc.Digest.String()]
	if !ok {
		manifestData, err := content.FetchAll(ctx, repo, desc)
		if err != nil {
			return name, version
		}
		var manifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return name, version
		}
		config = manifest.Config
	}
	configData, err := content.FetchAll(ctx, repo, config)
	if err != nil {
		return name, version
	}
	var meta struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(configData, &meta); err == nil {
		if name == "" {
			name = meta.Name
		}
		if version == "" {
			version = meta.Version
		}
	}
	return name, version
}

// describeCandidates renders chart candidates as name:version (falling back to
// the digest) for disambiguation error messages.
func describeCandidates(candidates []chartCandidate) string {
	parts := make([]string, 0, len(candidates))
	for _, c := range candidates {
		switch {
		case c.name == "":
			parts = append(parts, c.desc.Digest.String())
		case c.version == "":
			parts = append(parts, c.name)
		default:
			parts = append(parts, c.name+":"+c.version)
		}
	}
	return strings.Join(parts, ", ")
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
			selectedManifest, err := c.resolveFromIndex(ctx, repository, resolvedDesc, options.ArtifactType, options.Selectors)
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
