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

package repo // import "helm.sh/helm/pkg/repo"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	orascontent "github.com/deislabs/oras/pkg/content"
	"github.com/deislabs/oras/pkg/oras"
	"github.com/gosuri/uitable"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
)

// VerificationStrategy describes a strategy for determining whether to verify a chart.
type VerificationStrategy int

const (
	// VerifyNever will skip all verification of a chart.
	VerifyNever VerificationStrategy = 1 << iota
	// VerifyIfPossible will attempt a verification, it will not error if verification
	// data is missing. But it will not stop processing if verification fails.
	VerifyIfPossible
	// VerifyAlways will always attempt a verification, and will fail if the
	// verification fails.
	VerifyAlways
	// VerifyLater will fetch verification data, but not do any verification.
	// This is to accommodate the case where another step of the process will
	// perform verification.
	VerifyLater
)

// ClientOptions is used to construct a new client
type ClientOptions struct {
	Out          io.Writer
	CacheRootDir string
	// VerificationStrategy indicates what verification strategy to use.
	//
	// NOTE(bacongobbler): this is a no-op for now
	VerificationStrategy VerificationStrategy
}

// Client works with OCI-compliant registries and local Helm chart cache
type Client struct {
	out                  io.Writer
	resolver             *Resolver
	verificationStrategy VerificationStrategy
	cache                *filesystemCache // TODO: something more robust
}

// NewClient returns a new registry client with config
func NewClient(options *ClientOptions) *Client {
	return &Client{
		out:                  options.Out,
		resolver:             newResolver(docker.ResolverOptions{}),
		verificationStrategy: options.VerificationStrategy,
		cache: &filesystemCache{
			out:     options.Out,
			rootDir: options.CacheRootDir,
			store:   orascontent.NewMemoryStore(),
		},
	}
}

// PushChart uploads a chart to a registry
func (c *Client) PushChart(ref reference.Spec) error {
	ref, err := c.setDefaultTag(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "The push refers to repository [%s]\n", ref.Locator)
	layers, err := c.cache.LoadReference(ref)
	if err != nil {
		return err
	}
	err = oras.Push(context.Background(), c.resolver, ref.String(), c.cache.store, layers)
	if err != nil {
		return err
	}
	var totalSize int64
	for _, layer := range layers {
		totalSize += layer.Size
	}
	fmt.Fprintf(c.out,
		"%s: pushed to remote (%d layers, %s total)\n", ref.Object, len(layers), byteCountBinary(totalSize))
	return nil
}

// PullChart downloads a chart from a registry
func (c *Client) PullChart(ref reference.Spec) error {
	ref, err := c.setDefaultTag(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: Pulling %s\n", ref.Locator, ref.Object)
	layers, err := oras.Pull(context.Background(), c.resolver, ref.String(), c.cache.store, KnownMediaTypes()...)
	if err != nil {
		if err == reference.ErrObjectRequired {
			return fmt.Errorf(`chart "%s" not found in repository`, ref.String())
		}
		return err
	}

	exists, err := c.cache.StoreReference(ref, layers)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintf(c.out, "Status: Downloaded newer chart for %s:%s\n", ref.Locator, ref.Object)
	} else {
		fmt.Fprintf(c.out, "Status: Chart is up to date for %s:%s\n", ref.Locator, ref.Object)
	}

	// TODO(bacongobbler): do we store and fetch the provenance file from one of the layers
	// and verify it here, or do we sign the manifest using Notary and let oras handle
	// the verification step?
	//
	// https://github.com/containerd/containerd/issues/395#issuecomment-268105862

	return nil
}

// SaveChart stores a copy of chart in local cache
func (c *Client) SaveChart(ch *chart.Chart, registry string) error {
	// extract/separate the name and version from other metadata
	if ch.Metadata == nil {
		return errors.New("chart does not contain metadata")
	}
	name := ch.Metadata.Name
	version := ch.Metadata.Version

	ref, err := reference.Parse(path.Join(registry, fmt.Sprintf("%s:%s", name, version)))
	if err != nil {
		return err
	}

	layers, err := c.cache.ChartToLayers(ch)
	if err != nil {
		return err
	}
	_, err = c.cache.StoreReference(ref, layers)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: saved\n", ref.String())
	return nil
}

// LoadChart retrieves a chart object by reference
func (c *Client) LoadChart(ref reference.Spec) (*chart.Chart, error) {
	ref, err := c.setDefaultTag(ref)
	if err != nil {
		return nil, err
	}
	layers, err := c.cache.LoadReference(ref)
	if err != nil {
		return nil, err
	}
	ch, err := c.cache.LayersToChart(layers)
	return ch, err
}

// RemoveChart deletes a locally saved chart
func (c *Client) RemoveChart(ref reference.Spec) error {
	ref, err := c.setDefaultTag(ref)
	if err != nil {
		return err
	}
	err = c.cache.DeleteReference(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: removed\n", ref.Object)
	return err
}

// PrintChartTable prints a list of locally stored charts
func (c *Client) PrintChartTable() error {
	table := uitable.New()
	table.MaxColWidth = 60
	table.AddRow("REF", "NAME", "VERSION", "DIGEST", "SIZE", "CREATED")
	rows, err := c.cache.TableRows()
	if err != nil {
		return err
	}
	for _, row := range rows {
		table.AddRow(row...)
	}
	fmt.Fprintln(c.out, table.String())
	return nil
}

func (c *Client) setDefaultTag(ref reference.Spec) (reference.Spec, error) {

	// skip if the tag is already set
	if ref.Object != "" {
		return ref, nil
	}

	tags, err := c.FetchTags(ref.String())
	if err != nil {
		return reference.Spec{}, err
	}
	if len(tags) == 0 {
		return reference.Spec{}, fmt.Errorf("no tags were found for %s", ref.Locator)
	}

	// we need to create a new reference as the digest has changed.
	newRef, err := reference.Parse(fmt.Sprintf("%s:%s", ref.String(), tags[len(tags)-1]))
	if err != nil {
		return reference.Spec{}, err
	}

	return newRef, err
}

// FindChart finds a chart in the given chart repository.
func (c *Client) FindChart(ref reference.Spec) error {
	r, err := c.setDefaultTag(ref)
	if err != nil {
		return fmt.Errorf(`chart "%s" not found in repository`, ref.String())
	}
	return c.PullChart(r)
}

// FetchTags fetches the tags for a particular repository.
func (c *Client) FetchTags(repo string) ([]string, error) {
	ctx := context.Background()

	fetcher, err := c.tagFetcher(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get fetcher for %q: %v", repo, err)
	}

	buf := bytes.NewBuffer(nil)
	handlers := images.Handlers(
		fetchHandler(buf, fetcher),
	)

	if err := images.Dispatch(ctx, handlers, ocispec.Descriptor{}); err != nil {
		return nil, err
	}

	var resp tagsResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(resp.Tags)))

	return resp.Tags, nil
}

func (c *Client) tagFetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return nil, err
	}

	base, err := c.resolver.base(refspec)
	if err != nil {
		return nil, err
	}

	return tagFetcher{
		baseResolver: base,
	}, nil
}

func fetchHandler(w io.Writer, fetcher remotes.Fetcher) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
		rc, err := fetcher.Fetch(ctx, desc)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		_, err = io.Copy(w, rc)
		return nil, err
	}
}
