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

package action

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"helm.sh/helm/v3/pkg/cli/output"

	"github.com/gosuri/uitable"
	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// GetDeployed is the action for checking the named release's deployed resource list. It is the implementation
// of 'helm get deployed' subcommand.
//
// Example output:
//
//	NAMESPACE     	NAME                     	API_VERSION	AGE
//	              	namespaces/thousand-sunny	v1         	2m
//	thousand-sunny	configmaps/nami          	v1         	2m
//	thousand-sunny	services/zoro            	v1         	2m
//	thousand-sunny	deployments/luffy        	apps/v1    	2m
type GetDeployed struct {
	cfg *Configuration
}

// NewGetDeployed creates a new GetDeployed object with the input configuration.
func NewGetDeployed(cfg *Configuration) *GetDeployed {
	return &GetDeployed{
		cfg: cfg,
	}
}

// Run executes 'helm get deployed' against the named release.
func (g *GetDeployed) Run(name string) ([]ResourceElement, error) {
	// Check if cluster is reachable from the client
	if err := g.cfg.KubeClient.IsReachable(); err != nil {
		return nil, fmt.Errorf("cluster is not reachable: %w", err)
	}

	// Get the release details. The revision is set to 0 to get the latest revision of the release.
	release, err := g.cfg.releaseContent(name, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release content: %w", err)
	}

	// Fetch the REST mapper
	mapper, err := g.cfg.RESTClientGetter.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("failed to extract the REST mapper: %v", err)
	}

	// Create function to iterate over all the resources in the release manifest
	resourceList := make([]ResourceElement, 0)
	listResourcesFn := kio.FilterFunc(func(resources []*yaml.RNode) ([]*yaml.RNode, error) {
		// Iterate over the resource in manifest YAML
		for _, manifest := range resources {
			// Process resource record for "helm get deployed"
			resource, err := g.processResourceRecord(manifest, mapper)
			if err != nil {
				return nil, err
			}

			resourceList = append(resourceList, *resource)
		}

		// The current command shouldn't alter the list of resources. Hence returning resources list as it.
		return resources, nil
	})

	// Run the manifest YAML through the function to process the resources list
	err = kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: strings.NewReader(release.Manifest)}},
		Filters: []kio.Filter{listResourcesFn},
	}.Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to process release manifests: %w", err)
	}

	return resourceList, nil
}

// processResourceRecord processes the manifest YAML node in the record format required for resourceListWriter (i.e,
// output of `helm get deployed` command).
func (g *GetDeployed) processResourceRecord(manifest *yaml.RNode, mapper meta.RESTMapper) (*ResourceElement, error) {
	// Parse manifest YAML node as string
	manifestStr, err := manifest.String()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the string format of the manifest: %v", err)
	}

	// Build resource list required for Helm kube client
	filter, err := g.cfg.KubeClient.Build(bytes.NewBufferString(manifestStr), false)
	if err != nil {
		return nil, fmt.Errorf("failed to build resource list: %w", err)
	}

	// Fetch the resources from the Kubernetes cluster based on the resource list built above
	//
	// Note: processResourceRecord is for a single record/resource. However, Get() returns resources in a slice with
	// the current record.
	list, err := g.cfg.KubeClient.Get(filter, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get the resource from cluster: %w", err)
	}

	var (
		resourceObj runtime.Object
		objMeta     metav1.Object
	)

	// Extract the resource object and its metadata from the list of resources. Note: Though Get() returns a list of
	// resources, it only consists of one resource matching the resource name since it is filtered based on a single
	// resource's manifest.
	err = func() error {
		var ok bool
		for _, objects := range list {
			for _, obj := range objects {
				objMeta, ok = obj.(metav1.Object)
				if !ok {
					return fmt.Errorf("object does not implement metav1.Object interface")
				}

				if objMeta.GetName() != manifest.GetName() {
					continue
				}

				resourceObj = obj

				return nil
			}
		}

		return fmt.Errorf("failed to find resource %q in the list", manifest.GetName())
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to get the REST mapping for the resource: %v", err)
	}

	// Fetch the GVR mapping from Kubernetes REST mapper
	resourceMapping, err := restMapping(resourceObj, mapper)
	if err != nil {
		return nil, fmt.Errorf("failed to get the REST mapping for the resource: %v", err)
	}

	// Format resource record
	return &ResourceElement{
		Resource:          resourceMapping.Resource.Resource,
		Name:              manifest.GetName(),
		Namespace:         objMeta.GetNamespace(),
		APIVersion:        manifest.GetApiVersion(),
		CreationTimestamp: objMeta.GetCreationTimestamp(),
	}, nil
}

// restMapping returns the GVR mapping from Kubernetes REST mapper
func restMapping(obj runtime.Object, mapper meta.RESTMapper) (*meta.RESTMapping, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to find RESTMapping: %v", err)
	}

	return mapping, nil
}

type ResourceElement struct {
	Name              string      `json:"name"`              // Resource's name
	Namespace         string      `json:"namespace"`         // Resource's namespace
	APIVersion        string      `json:"apiVersion"`        // Resource's group-version
	Resource          string      `json:"resource"`          // Resource type (eg. pods, deployments, etc.)
	CreationTimestamp metav1.Time `json:"creationTimestamp"` // Resource creation timestamp
}

type resourceListWriter struct {
	releases  []ResourceElement // Resources list
	noHeaders bool              // Toggle to disable headers in tabular format
}

// NewResourceListWriter creates a output writer for Kubernetes resources to be listed with 'helm get deployed'
func NewResourceListWriter(resources []ResourceElement, noHeaders bool) output.Writer {
	return &resourceListWriter{resources, noHeaders}
}

// WriteTable prints the resources list in a tabular format
func (r *resourceListWriter) WriteTable(out io.Writer) error {
	// Create table writer
	table := uitable.New()

	// Add headers if enabled
	if !r.noHeaders {
		table.AddRow("NAMESPACE", "NAME", "API_VERSION", "AGE")
	}

	// Add resources to table
	for _, r := range r.releases {
		table.AddRow(
			r.Namespace,                              // Namespace
			fmt.Sprintf("%s/%s", r.Resource, r.Name), // Name
			r.APIVersion,                             // API version
			metatable.ConvertToHumanReadableDateType(r.CreationTimestamp), // Age
		)
	}

	// Format the table and write to output writer
	return output.EncodeTable(out, table)
}

// WriteTable prints the resources list in a JSON format
func (r *resourceListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, r.releases)
}

// WriteTable prints the resources list in a YAML format
func (r *resourceListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, r.releases)
}
