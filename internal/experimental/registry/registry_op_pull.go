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
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"

	"helm.sh/helm/v3/pkg/chart"
)

// Pull downloads a chart from a registry
func (c *Client) Pull(ref string, options ...PullOption) (*PullResult, error) {
	operation := &pullOperation{
		withChart: true, // By default, always download the chart layer
	}
	for _, option := range options {
		option(operation)
	}
	if !operation.withChart && !operation.withProv {
		return nil, errors.New(
			"must specify at least one layer to pull (chart/prov)")
	}
	store := content.NewMemoryStore()
	allowedMediaTypes := []string{
		ConfigMediaType,
	}
	minNumDescriptors := 1 // 1 for the config
	if operation.withChart {
		minNumDescriptors++
		allowedMediaTypes = append(allowedMediaTypes, ChartLayerMediaType)
	}
	if operation.withProv {
		if !operation.ignoreMissingProv {
			minNumDescriptors++
		}
		allowedMediaTypes = append(allowedMediaTypes, ProvLayerMediaType)
	}
	manifest, descriptors, err := oras.Pull(ctx(c.out, c.debug), c.resolver, ref, store,
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(allowedMediaTypes))
	if err != nil {
		return nil, err
	}
	numDescriptors := len(descriptors)
	if numDescriptors < minNumDescriptors {
		return nil, errors.New(
			fmt.Sprintf("manifest does not contain minimum number of descriptors (%d), descriptors found: %d",
				minNumDescriptors, numDescriptors))
	}
	var configDescriptor *ocispec.Descriptor
	var chartDescriptor *ocispec.Descriptor
	var provDescriptor *ocispec.Descriptor
	for _, descriptor := range descriptors {
		d := descriptor
		switch d.MediaType {
		case ConfigMediaType:
			configDescriptor = &d
		case ChartLayerMediaType:
			chartDescriptor = &d
		case ProvLayerMediaType:
			provDescriptor = &d
		}
	}
	if configDescriptor == nil {
		return nil, errors.New(
			fmt.Sprintf("could not load config with mediatype %s", ConfigMediaType))
	}
	if operation.withChart && chartDescriptor == nil {
		return nil, errors.New(
			fmt.Sprintf("manifest does not contain a layer with mediatype %s",
				ChartLayerMediaType))
	}
	var provMissing bool
	if operation.withProv && provDescriptor == nil {
		if operation.ignoreMissingProv {
			provMissing = true
		} else {
			return nil, errors.New(
				fmt.Sprintf("manifest does not contain a layer with mediatype %s",
					ProvLayerMediaType))
		}
	}
	_, manifestData, ok := store.Get(manifest)
	if !ok {
		return nil, errors.Errorf("Unable to retrieve blob with digest %s", manifest.Digest)
	}
	_, configData, ok := store.Get(*configDescriptor)
	if !ok {
		return nil, errors.Errorf("Unable to retrieve blob with digest %s", configDescriptor.Digest)
	}
	var meta *chart.Metadata
	if err := json.Unmarshal(configData, &meta); err != nil {
		return nil, err
	}
	var chartData []byte
	if operation.withChart {
		var ok bool
		_, chartData, ok = store.Get(*chartDescriptor)
		if !ok {
			return nil, errors.Errorf("Unable to retrieve blob with digest %s", chartDescriptor.Digest)
		}
	}
	var provData []byte
	if operation.withProv && !provMissing {
		var ok bool
		_, provData, ok = store.Get(*provDescriptor)
		if !ok {
			return nil, errors.Errorf("Unable to retrieve blob with digest %s", provDescriptor.Digest)
		}
	}
	result := &PullResult{
		Manifest: &descriptorPullSummary{
			Data:   manifestData,
			Digest: manifest.Digest.String(),
			Size:   manifest.Size,
		},
		Config: &descriptorPullSummary{
			Data:   configData,
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		Chart: &descriptorPullSummaryWithMeta{
			Meta: meta,
		},
		Prov: &descriptorPullSummary{}, // prevent nil references
		Ref:  ref,
	}
	if chartData != nil {
		result.Chart.Data = chartData
		result.Chart.Digest = chartDescriptor.Digest.String()
		result.Chart.Size = chartDescriptor.Size
	}
	if provData != nil {
		result.Prov = &descriptorPullSummary{
			Data:   provData,
			Digest: provDescriptor.Digest.String(),
			Size:   provDescriptor.Size,
		}
	}
	fmt.Fprintf(c.out, "Pulled: %s\n", result.Ref)
	fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	return result, nil
}
