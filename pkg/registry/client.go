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

package registry // import "helm.sh/helm/pkg/registry"

import (
	"context"
	"fmt"
	"io"

	orascontent "github.com/deislabs/oras/pkg/content"
	orascontext "github.com/deislabs/oras/pkg/context"
	"github.com/deislabs/oras/pkg/oras"
	"github.com/gosuri/uitable"
	"github.com/sirupsen/logrus"

	"helm.sh/helm/pkg/chart"
)

const (
	CredentialsFileBasename = "config.json"
)

type (
	// ClientOptions is used to construct a new client
	ClientOptions struct {
		Debug        bool
		Out          io.Writer
		Authorizer   Authorizer
		Resolver     Resolver
		CacheRootDir string
	}

	// Client works with OCI-compliant registries and local Helm chart cache
	Client struct {
		debug      bool
		out        io.Writer
		authorizer Authorizer
		resolver   Resolver
		cache      *filesystemCache // TODO: something more robust
	}
)

// NewClient returns a new registry client with config
func NewClient(options *ClientOptions) *Client {
	return &Client{
		debug:      options.Debug,
		out:        options.Out,
		resolver:   options.Resolver,
		authorizer: options.Authorizer,
		cache: &filesystemCache{
			out:     options.Out,
			rootDir: options.CacheRootDir,
			store:   orascontent.NewMemoryStore(),
		},
	}
}

// Login logs into a registry
func (c *Client) Login(hostname string, username string, password string) error {
	err := c.authorizer.Login(c.newContext(), hostname, username, password)
	if err != nil {
		return err
	}
	fmt.Fprint(c.out, "Login succeeded\n")
	return nil
}

// Logout logs out of a registry
func (c *Client) Logout(hostname string) error {
	err := c.authorizer.Logout(c.newContext(), hostname)
	if err != nil {
		return err
	}
	fmt.Fprint(c.out, "Logout succeeded\n")
	return nil
}

// PushChart uploads a chart to a registry
func (c *Client) PushChart(ref *Reference) error {
	c.setDefaultTag(ref)
	fmt.Fprintf(c.out, "The push refers to repository [%s]\n", ref.Repo)
	layers, err := c.cache.LoadReference(ref)
	if err != nil {
		return err
	}
	_, err = oras.Push(c.newContext(), c.resolver, ref.String(), c.cache.store, layers,
		oras.WithConfigMediaType(HelmChartConfigMediaType))
	if err != nil {
		return err
	}
	var totalSize int64
	for _, layer := range layers {
		totalSize += layer.Size
	}
	fmt.Fprintf(c.out,
		"%s: pushed to remote (%d layers, %s total)\n", ref.Tag, len(layers), byteCountBinary(totalSize))
	return nil
}

// PullChart downloads a chart from a registry
func (c *Client) PullChart(ref *Reference) error {
	c.setDefaultTag(ref)
	fmt.Fprintf(c.out, "%s: Pulling from %s\n", ref.Tag, ref.Repo)
	_, layers, err := oras.Pull(c.newContext(), c.resolver, ref.String(), c.cache.store, oras.WithAllowedMediaTypes(KnownMediaTypes()))
	if err != nil {
		return err
	}
	exists, err := c.cache.StoreReference(ref, layers)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintf(c.out, "Status: Downloaded newer chart for %s:%s\n", ref.Repo, ref.Tag)
	} else {
		fmt.Fprintf(c.out, "Status: Chart is up to date for %s:%s\n", ref.Repo, ref.Tag)
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
	fmt.Fprintf(c.out, "%s: saved\n", ref.Tag)
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
	fmt.Fprintf(c.out, "%s: removed\n", ref.Tag)
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
	if ref.Tag == "" {
		ref.Tag = HelmChartDefaultTag
		fmt.Fprintf(c.out, "Using default tag: %s\n", HelmChartDefaultTag)
	}
}

// disable verbose logging coming from ORAS unless debug is enabled
func (c *Client) newContext() context.Context {
	if !c.debug {
		return orascontext.Background()
	}
	ctx := orascontext.WithLoggerFromWriter(context.Background(), c.out)
	orascontext.GetLogger(ctx).Logger.SetLevel(logrus.DebugLevel)
	return ctx
}
