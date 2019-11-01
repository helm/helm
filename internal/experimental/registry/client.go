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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"

	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/deislabs/oras/pkg/oras"
	"github.com/gosuri/uitable"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/helmpath"
)

const (
	// CredentialsFileBasename is the filename for auth credentials file
	CredentialsFileBasename = "config.json"
)

type (
	// Client works with OCI-compliant registries and local Helm chart cache
	Client struct {
		debug      bool
		out        io.Writer
		authorizer *Authorizer
		resolver   *Resolver
		cache      *Cache
	}
)

// NewClient returns a new registry client with config
func NewClient(opts ...ClientOption) (*Client, error) {
	client := &Client{
		out: ioutil.Discard,
	}
	for _, opt := range opts {
		opt(client)
	}
	// set defaults if fields are missing
	if client.authorizer == nil {
		credentialsFile := helmpath.CachePath("registry", CredentialsFileBasename)
		authClient, err := auth.NewClient(credentialsFile)
		if err != nil {
			return nil, err
		}
		client.authorizer = &Authorizer{
			Client: authClient,
		}
	}
	if client.resolver == nil {
		resolver, err := client.authorizer.Resolver(context.Background(), http.DefaultClient, false)
		if err != nil {
			return nil, err
		}
		client.resolver = &Resolver{
			Resolver: resolver,
		}
	}
	if client.cache == nil {
		cache, err := NewCache(
			CacheOptDebug(client.debug),
			CacheOptWriter(client.out),
			CacheOptRoot(helmpath.CachePath("registry", CacheRootDir)),
		)
		if err != nil {
			return nil, err
		}
		client.cache = cache
	}
	return client, nil
}

// Login logs into a registry
func (c *Client) Login(hostname string, username string, password string, insecure bool) error {
	err := c.authorizer.Login(ctx(c.out, c.debug), hostname, username, password, insecure)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Login succeeded\n")
	return nil
}

// Logout logs out of a registry
func (c *Client) Logout(hostname string) error {
	err := c.authorizer.Logout(ctx(c.out, c.debug), hostname)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.out, "Logout succeeded")
	return nil
}

// PushChart uploads a chart to a registry
func (c *Client) PushChart(ref *Reference) error {
	r, err := c.cache.FetchReference(ref)
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New(fmt.Sprintf("Chart not found: %s", r.Name))
	}
	fmt.Fprintf(c.out, "The push refers to repository [%s]\n", r.Repo)
	c.printCacheRefSummary(r)
	layers := []ocispec.Descriptor{*r.ContentLayer}
	_, err = oras.Push(ctx(c.out, c.debug), c.resolver, r.Name, c.cache.Provider(), layers,
		oras.WithConfig(*r.Config), oras.WithNameValidation(nil))
	if err != nil {
		return err
	}
	s := ""
	numLayers := len(layers)
	if 1 < numLayers {
		s = "s"
	}
	fmt.Fprintf(c.out,
		"%s: pushed to remote (%d layer%s, %s total)\n", r.Tag, numLayers, s, byteCountBinary(r.Size))
	return nil
}

// PullChart downloads a chart from a registry
func (c *Client) PullChart(ref *Reference) error {
	if ref.Tag == "" {
		return errors.New("tag explicitly required")
	}
	existing, err := c.cache.FetchReference(ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: Pulling from %s\n", ref.Tag, ref.Repo)
	manifest, _, err := oras.Pull(ctx(c.out, c.debug), c.resolver, ref.FullName(), c.cache.Ingester(),
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(KnownMediaTypes()),
		oras.WithContentProvideIngester(c.cache.ProvideIngester()))
	if err != nil {
		return err
	}
	err = c.cache.AddManifest(ref, &manifest)
	if err != nil {
		return err
	}
	r, err := c.cache.FetchReference(ref)
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New(fmt.Sprintf("Chart not found: %s", r.Name))
	}
	c.printCacheRefSummary(r)
	if !existing.Exists {
		fmt.Fprintf(c.out, "Status: Downloaded newer chart for %s\n", ref.FullName())
	} else {
		fmt.Fprintf(c.out, "Status: Chart is up to date for %s\n", ref.FullName())
	}
	return err
}

// SaveChart stores a copy of chart in local cache
func (c *Client) SaveChart(ch *chart.Chart, ref *Reference) error {
	r, err := c.cache.StoreReference(ref, ch)
	if err != nil {
		return err
	}
	c.printCacheRefSummary(r)
	err = c.cache.AddManifest(ref, r.Manifest)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: saved\n", r.Tag)
	return nil
}

// LoadChart retrieves a chart object by reference
func (c *Client) LoadChart(ref *Reference) (*chart.Chart, error) {
	r, err := c.cache.FetchReference(ref)
	if err != nil {
		return nil, err
	}
	if !r.Exists {
		return nil, errors.New(fmt.Sprintf("Chart not found: %s", ref.FullName()))
	}
	c.printCacheRefSummary(r)
	return r.Chart, nil
}

// RemoveChart deletes a locally saved chart
func (c *Client) RemoveChart(ref *Reference) error {
	r, err := c.cache.DeleteReference(ref)
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New(fmt.Sprintf("Chart not found: %s", ref.FullName()))
	}
	fmt.Fprintf(c.out, "%s: removed\n", r.Tag)
	return nil
}

// PrintChartTable prints a list of locally stored charts
func (c *Client) PrintChartTable() error {
	table := uitable.New()
	table.MaxColWidth = 60
	table.AddRow("REF", "NAME", "VERSION", "DIGEST", "SIZE", "CREATED")
	rows, err := c.getChartTableRows()
	if err != nil {
		return err
	}
	for _, row := range rows {
		table.AddRow(row...)
	}
	fmt.Fprintln(c.out, table.String())
	return nil
}

// printCacheRefSummary prints out chart ref summary
func (c *Client) printCacheRefSummary(r *CacheRefSummary) {
	fmt.Fprintf(c.out, "ref:     %s\n", r.Name)
	fmt.Fprintf(c.out, "digest:  %s\n", r.Digest.Hex())
	fmt.Fprintf(c.out, "size:    %s\n", byteCountBinary(r.Size))
	fmt.Fprintf(c.out, "name:    %s\n", r.Chart.Metadata.Name)
	fmt.Fprintf(c.out, "version: %s\n", r.Chart.Metadata.Version)
}

// getChartTableRows returns rows in uitable-friendly format
func (c *Client) getChartTableRows() ([][]interface{}, error) {
	rr, err := c.cache.ListReferences()
	if err != nil {
		return nil, err
	}
	refsMap := map[string]map[string]string{}
	for _, r := range rr {
		refsMap[r.Name] = map[string]string{
			"name":    r.Chart.Metadata.Name,
			"version": r.Chart.Metadata.Version,
			"digest":  shortDigest(r.Digest.Hex()),
			"size":    byteCountBinary(r.Size),
			"created": timeAgo(r.CreatedAt),
		}
	}
	// Sort and convert to format expected by uitable
	rows := make([][]interface{}, len(refsMap))
	keys := make([]string, 0, len(refsMap))
	for key := range refsMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		rows[i] = make([]interface{}, 6)
		rows[i][0] = key
		ref := refsMap[key]
		for j, k := range []string{"name", "version", "digest", "size", "created"} {
			rows[i][j+1] = ref[k]
		}
	}
	return rows, nil
}
