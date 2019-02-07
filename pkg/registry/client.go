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

package registry // import "k8s.io/helm/pkg/registry"

import (
	"context"
	"fmt"
	"io"

	orascontent "github.com/deislabs/oras/pkg/content"
	"github.com/deislabs/oras/pkg/oras"
	"github.com/gosuri/uitable"

	"k8s.io/helm/pkg/chart"
)

type (
	// ClientOptions is used to construct a new client
	ClientOptions struct {
		Out          io.Writer
		Resolver     Resolver
		CacheRootDir string
	}

	// Client works with OCI-compliant registries and local Helm chart cache
	Client struct {
		out      io.Writer
		resolver Resolver
		cache    *filesystemCache // TODO: something more robust
	}
)

// NewClient returns a new registry client with config
func NewClient(options *ClientOptions) *Client {
	return &Client{
		out:      options.Out,
		resolver: options.Resolver,
		cache: &filesystemCache{
			out:     options.Out,
			rootDir: options.CacheRootDir,
			store:   orascontent.NewMemoryStore(),
		},
	}
}

// PushChart uploads a chart to a registry
func (c *Client) PushChart(ref *Reference) error {
	c.setDefaultTag(ref)
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
func (c *Client) PullChart(ref *Reference) error {
	c.setDefaultTag(ref)
	fmt.Fprintf(c.out, "%s: Pulling from %s\n", ref.Object, ref.Locator)
	layers, err := oras.Pull(context.Background(), c.resolver, ref.String(), c.cache.store, KnownMediaTypes()...)
	if err != nil {
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
	return nil
}

// SaveChart stores a copy of chart in local cache
func (c *Client) SaveChart(ch *chart.Chart, ref *Reference) error {
	c.setDefaultTag(ref)
	layers, err := c.cache.ChartToLayers(ch)
	if err != nil {
		return err
	}
	_, err = c.cache.StoreReference(ref, layers)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s: saved\n", ref.Object)
	return nil
}

// LoadChart retrieves a chart object by reference
func (c *Client) LoadChart(ref *Reference) (*chart.Chart, error) {
	c.setDefaultTag(ref)
	layers, err := c.cache.LoadReference(ref)
	if err != nil {
		return nil, err
	}
	ch, err := c.cache.LayersToChart(layers)
	return ch, err
}

// RemoveChart deletes a locally saved chart
func (c *Client) RemoveChart(ref *Reference) error {
	c.setDefaultTag(ref)
	err := c.cache.DeleteReference(ref)
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

func (c *Client) setDefaultTag(ref *Reference) {
	if ref.Object == "" {
		ref.Object = HelmChartDefaultTag
		fmt.Fprintf(c.out, "Using default tag: %s\n", HelmChartDefaultTag)
	}
}
