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

package getter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/experimental/registry"
)

// OCIGetter is the default HTTP(/S) backend handler
type OCIGetter struct {
	opts options
}

// Get performs a Get from repo.Getter and returns the body.
func (g *OCIGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *OCIGetter) get(href string) (*bytes.Buffer, error) {
	client := g.opts.registryClient

	ref := strings.TrimPrefix(href, fmt.Sprintf("%s://", registry.OCIScheme))

	var pullOpts []registry.PullOption
	requestingProv := strings.HasSuffix(ref, ".prov")
	if requestingProv {
		ref = strings.TrimSuffix(ref, ".prov")
		pullOpts = append(pullOpts,
			registry.PullOptWithChart(false),
			registry.PullOptWithProv(true))
	}

	// Retrieve list of repository tags
	tags, err := client.Tags(ref)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, errors.Errorf("Unable to locate any tags in provided repository: %s", ref)
	}

	// Determine if version provided
	// If empty, try to get the highest available tag
	// If exact version, try to find it
	// If semver constraint string, try to find a match
	providedVersion := g.opts.version

	tag, err := registry.GetTagMatchingVersionOrConstraint(tags, providedVersion)
	if err != nil {
		return nil, err
	}

	ref = fmt.Sprintf("%s:%s", ref, tag)

	result, err := client.Pull(ref, pullOpts...)
	if err != nil {
		return nil, err
	}

	if requestingProv {
		return bytes.NewBuffer(result.Prov.Data), nil
	}
	return bytes.NewBuffer(result.Chart.Data), nil
}

// NewOCIGetter constructs a valid http/https client as a Getter
func NewOCIGetter(ops ...Option) (Getter, error) {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}

	client := OCIGetter{
		opts: options{
			registryClient: registryClient,
		},
	}

	for _, opt := range ops {
		opt(&client.opts)
	}

	return &client, nil
}
