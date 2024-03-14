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
	"context"
	"fmt"
	"io"
	"strings"

	"helm.sh/helm/v3/pkg/cli/output"

	"github.com/gosuri/uitable"
	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// GetDeployed is the action for checking the named release's deployed resources.
//
// It provides the implementation of 'helm get deployed'.
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
	ctx := context.Background()

	// Check if cluster is reachable from the client
	if err := g.cfg.KubeClient.IsReachable(); err != nil {
		return nil, fmt.Errorf("kube client is not reachable: %w", err)
	}

	// Load the REST config for client
	config, err := g.cfg.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get the REST config: %w", err)
	}

	// Create a dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create a dynamic client: %w", err)
	}

	// Create a REST mapper from config
	restMapper, err := g.cfg.RESTClientGetter.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("failed to get the REST mapper: %w", err)
	}

	// Default namespace set in kube client
	defaultNamespace := g.cfg.KubeClient.GetNamespace()

	// Get the release details.
	//
	// The revision is set to 0 to get the latest revision of the release.
	release, err := g.cfg.releaseContent(name, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release content: %w", err)
	}

	// Create function to iterate over all the resources in the release manifest
	resourceList := make([]ResourceElement, 0)
	listResourcesFn := kio.FilterFunc(func(resources []*yaml.RNode) ([]*yaml.RNode, error) {
		// Iterate over the resource in manifest YAML
		for _, manifest := range resources {
			// Process resource to be printable by the "helm get deployed" command's output writer
			resource, err := processResourceForGetDeployed(ctx, manifest, dynamicClient, restMapper, defaultNamespace)
			if err != nil {
				return nil, err
			}

			// Add current resource to list
			resourceList = append(resourceList, *resource)
		}

		// Return resources as is, as this function is only expected to read the resource manifest
		return resources, nil
	})

	// Run the manifest YAML through the function to process the resources list
	err = kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: strings.NewReader(release.Manifest)}},
		Filters: []kio.Filter{listResourcesFn},
	}.Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to run read and process release manifests: %w", err)
	}

	return resourceList, nil
}

// processResourceForGetDeployed processes resource to be printable by the "helm get deployed" command's output writer
func processResourceForGetDeployed(ctx context.Context, manifest *yaml.RNode, dynamicClient *dynamic.DynamicClient,
	restMapper meta.RESTMapper, defaultNamespace string) (*ResourceElement, error) {
	// Extract the resource's name field from manifest YAML
	name := manifest.GetName()
	if name == "" {
		return nil, fmt.Errorf("resource name not found in manifest: %v", manifest)
	}

	// Extract the resource's API version field from manifest YAML
	apiVersion := manifest.GetApiVersion()
	if apiVersion == "" {
		return nil, fmt.Errorf("resource api version not found in manifest: %v", manifest)
	}

	// Extract the resource's GVK
	gvk, err := extractGVK(manifest)
	if err != nil {
		return nil, err
	}

	// Extract the resource's namespace
	namespace, err := extractResourceNamespace(manifest, *gvk, restMapper, defaultNamespace)
	if err != nil {
		return nil, err
	}

	// Create a REST mapping for the resource and GVK
	restMappingForGVK, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get the REST mapping for GVK: %w", err)
	}

	// Get the resource's details from the cluster
	resource, err := getResource(ctx, dynamicClient, name, namespace, restMappingForGVK.Resource)
	if err != nil {
		return nil, err
	}

	return &ResourceElement{
		Resource:          restMappingForGVK.Resource.Resource,
		Name:              name,
		Namespace:         namespace,
		APIVersion:        apiVersion,
		CreationTimestamp: resource.GetCreationTimestamp(),
	}, nil
}

// getResource gets the Kubernetes resource using dynamic client.
func getResource(ctx context.Context, client *dynamic.DynamicClient, name, namespace string,
	gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	// If the namespace is not empty, it looks for a namespace-scoped resource. It is the responsibility of the caller
	// to provide the namespace value for the namespace-scoped resource even if it uses the default namespace.
	if namespace != "" {
		resource, err := client.Resource(gvr).
			Namespace(namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get the namespaced resource %q: %w", gvr, err)
		}

		return resource, nil
	}

	// Get cluster-scoped resource
	resource, err := client.Resource(gvr).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get the non-namespaced resource %q: %w", gvr, err)
	}

	return resource, nil
}

// extractGVK extracts group-version and kind from the manifest YAML, and forms a GVK out of it.
func extractGVK(manifest *yaml.RNode) (*schema.GroupVersionKind, error) {
	// Get the group-version field from manifest YAML
	gv := manifest.GetApiVersion()
	if gv == "" {
		return nil, fmt.Errorf("failed to get the resource's apiVersion from manifest: %v", manifest)
	}

	// Get the kind field from manifest YAML
	kind := manifest.GetKind()
	if kind == "" {
		return nil, fmt.Errorf("failed to get the resource's kind from manifest: %v", manifest)
	}

	// Create GVK out of group-version and kind from manifest
	gvk := schema.FromAPIVersionAndKind(gv, kind)

	return &gvk, nil
}

// extractResourceNamespace extracts resource name field ("manifest.namespace") from the manifest YAML, if the
// resource is namespace-scoped. For cluster-scoped, it returns empty string.
//
// Note: The YAML RNode should be of a single resource.
func extractResourceNamespace(manifest *yaml.RNode, gvk schema.GroupVersionKind, restMapper meta.RESTMapper,
	defaultNamespace string) (string, error) {
	// Extract the resource's namespace field from manifest YAML
	namespace := manifest.GetNamespace()
	if namespace != "" {
		return namespace, nil
	}

	// Check whether the current resource is namespace-scoped
	isResourceNamespaced, err := apiutil.IsGVKNamespaced(gvk, restMapper)
	if err != nil {
		return "", fmt.Errorf("failed to check if GVK is namespaced: %w", err)
	}

	// When the resource is namespace-scoped, and namespace field is missing (or empty) in the manifest, use the
	// default namespace. Note: The default namespace can be "default" or an overridden value in kube config.
	//
	// TODO :: Bhargav-InfraCloud :: If Helm manifest command (helm get manifest RELEASE_NAME) appends the namespace,
	// for templates where the namespace is not specified, may it be “default” or namespace value that is overridden in
	// kube config, assigning default namespace here can be removed. And maybe an error can be returned if it has an
	// empty namespace value even after that.
	if isResourceNamespaced {
		return defaultNamespace, nil
	}

	// When the resource is cluster-scoped, return empty string
	return "", nil
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
