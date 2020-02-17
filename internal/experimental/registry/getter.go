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
	"bytes"
	"net/url"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/getter"
)

// Getter is the HTTP(/S) backend handler for OCI image registries.
type Getter struct {
	Client *Client
}

func NewRegistryGetter(c *Client) *Getter {
	return &Getter{Client: c}
}

func NewRegistryGetterProvider(c *Client) getter.Provider {
	return getter.Provider{
		Schemes: []string{OCIProtocol},
		New: func(options ...getter.Option) (g getter.Getter, e error) {
			return NewRegistryGetter(c), nil
		},
	}
}

func (g *Getter) Get(href string, options ...getter.Option) (*bytes.Buffer, error) {
	u, err := url.Parse(href)

	if err != nil {
		return nil, err
	}

	ref, err := ParseReference(u.Host + u.Path)

	if err != nil {
		return nil, err
	}

	// first we'll pull the chart
	err = g.Client.PullChart(ref)

	if err != nil {
		return nil, err
	}

	// once we know we have the chart, we'll load up the chart
	c, err := g.Client.LoadChart(ref)

	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)

	// lastly, we'll write the tarred and gzipped contents of the chart to our output buffer
	err = chartutil.Write(c, buf)

	return buf, err
}
