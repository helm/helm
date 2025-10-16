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
	"io"
	"net/http"
	"sort"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
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
	authorizer         *Authorizer
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

// PullGeneric performs a generic OCI pull without artifact-specific assumptions
func (c *GenericClient) PullGeneric(ref string, options GenericPullOptions) (*GenericPullResult, error) {
	parsedRef, err := newReference(ref)
	if err != nil {
		return nil, err
	}

	memoryStore := memory.New()
	var descriptors []ocispec.Descriptor

	// Set up repository with authentication and configuration
	repository, err := remote.NewRepository(parsedRef.String())
	if err != nil {
		return nil, err
	}
	repository.PlainHTTP = c.plainHTTP
	repository.Client = c.authorizer

	ctx := context.Background()

	// Prepare allowed media types for filtering
	var allowedMediaTypes []string
	if len(options.AllowedMediaTypes) > 0 {
		allowedMediaTypes = make([]string, len(options.AllowedMediaTypes))
		copy(allowedMediaTypes, options.AllowedMediaTypes)
		sort.Strings(allowedMediaTypes)
	}

	var mu sync.Mutex
	manifest, err := oras.Copy(ctx, repository, parsedRef.String(), memoryStore, "", oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			PreCopy: func(ctx context.Context, desc ocispec.Descriptor) error {
				// Apply custom PreCopy function if provided
				if options.PreCopy != nil {
					if err := options.PreCopy(ctx, desc); err != nil {
						return err
					}
				}

				mediaType := desc.MediaType

				// Skip media types if specified
				for _, skipType := range options.SkipMediaTypes {
					if mediaType == skipType {
						return oras.SkipNode
					}
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
