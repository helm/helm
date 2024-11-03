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

package fake

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"helm.sh/helm/v3/pkg/kube"
)

// Options to control the fake behavior of PrintingKubeClient
type Options struct {
	GetReturnResourceMap    bool
	GetReturnError          bool
	BuildReturnResourceList bool
	BuildReturnError        bool
	IsReachableReturnsError bool
}

// PrintingKubeClient implements KubeClient, but simply prints the reader to
// the given output.
type PrintingKubeClient struct {
	Out     io.Writer
	Options *Options
}

var (
	ErrPrintingKubeClientNotReachable = errors.New("kubernetes cluster not reachable")
	ErrPrintingKubeClientBuildFailure = errors.New("failed to build resource list")
	ErrPrintingKubeClientGetFailure   = errors.New("failed to get resource")
)

// IsReachable checks if the cluster is reachable
func (p *PrintingKubeClient) IsReachable() error {
	if p.Options != nil && p.Options.IsReachableReturnsError {
		return ErrPrintingKubeClientNotReachable
	}

	return nil
}

// Create prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Create(resources kube.ResourceList) (*kube.Result, error) {
	_, err := io.Copy(p.Out, bufferize(resources))
	if err != nil {
		return nil, err
	}
	return &kube.Result{Created: resources}, nil
}

func (p *PrintingKubeClient) Get(resources kube.ResourceList, _ bool) (map[string][]runtime.Object, error) {
	_, err := io.Copy(p.Out, bufferize(resources))
	if err != nil {
		return nil, err
	}

	if p.Options != nil {
		if p.Options.GetReturnError {
			return nil, ErrPrintingKubeClientGetFailure
		}

		if p.Options.GetReturnResourceMap {
			result := make(map[string][]runtime.Object)
			for _, r := range resources {
				result[r.Name] = []runtime.Object{
					r.Object,
				}
			}

			return result, nil
		}
	}

	return make(map[string][]runtime.Object), nil
}

func (p *PrintingKubeClient) Wait(resources kube.ResourceList, _ time.Duration) error {
	_, err := io.Copy(p.Out, bufferize(resources))
	return err
}

func (p *PrintingKubeClient) WaitWithJobs(resources kube.ResourceList, _ time.Duration) error {
	_, err := io.Copy(p.Out, bufferize(resources))
	return err
}

func (p *PrintingKubeClient) WaitForDelete(resources kube.ResourceList, _ time.Duration) error {
	_, err := io.Copy(p.Out, bufferize(resources))
	return err
}

// Delete implements KubeClient delete.
//
// It only prints out the content to be deleted.
func (p *PrintingKubeClient) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	_, err := io.Copy(p.Out, bufferize(resources))
	if err != nil {
		return nil, []error{err}
	}
	return &kube.Result{Deleted: resources}, nil
}

// WatchUntilReady implements KubeClient WatchUntilReady.
func (p *PrintingKubeClient) WatchUntilReady(resources kube.ResourceList, _ time.Duration) error {
	_, err := io.Copy(p.Out, bufferize(resources))
	return err
}

// Update implements KubeClient Update.
func (p *PrintingKubeClient) Update(_, modified kube.ResourceList, _ bool) (*kube.Result, error) {
	_, err := io.Copy(p.Out, bufferize(modified))
	if err != nil {
		return nil, err
	}
	// TODO: This doesn't completely mock out have some that get created,
	// updated, and deleted. I don't think these are used in any unit tests, but
	// we may want to refactor a way to handle future tests
	return &kube.Result{Updated: modified}, nil
}

// Build implements KubeClient Build.
func (p *PrintingKubeClient) Build(in io.Reader, _ bool) (kube.ResourceList, error) {
	if p.Options != nil {
		if p.Options.BuildReturnError {
			return nil, ErrPrintingKubeClientBuildFailure
		}

		if p.Options.BuildReturnResourceList {
			manifest, err := (&kio.ByteReader{Reader: in}).Read()
			if err != nil {
				return nil, err
			}

			resources, err := parseResources(manifest)
			if err != nil {
				return nil, err
			}

			return resources, nil
		}
	}

	return []*resource.Info{}, nil
}

// BuildTable implements KubeClient BuildTable.
func (p *PrintingKubeClient) BuildTable(_ io.Reader, _ bool) (kube.ResourceList, error) {
	return []*resource.Info{}, nil
}

// WaitAndGetCompletedPodPhase implements KubeClient WaitAndGetCompletedPodPhase.
func (p *PrintingKubeClient) WaitAndGetCompletedPodPhase(_ string, _ time.Duration) (corev1.PodPhase, error) {
	return corev1.PodSucceeded, nil
}

// DeleteWithPropagationPolicy implements KubeClient delete.
//
// It only prints out the content to be deleted.
func (p *PrintingKubeClient) DeleteWithPropagationPolicy(resources kube.ResourceList, _ metav1.DeletionPropagation) (*kube.Result, []error) {
	_, err := io.Copy(p.Out, bufferize(resources))
	if err != nil {
		return nil, []error{err}
	}
	return &kube.Result{Deleted: resources}, nil
}

func bufferize(resources kube.ResourceList) io.Reader {
	var builder strings.Builder
	for _, info := range resources {
		builder.WriteString(info.String() + "\n")
	}
	return strings.NewReader(builder.String())
}

// parseResources parses Kubernetes manifest YAML as resources suitable for Helm
func parseResources(manifest []*yaml.RNode) ([]*resource.Info, error) {
	// Create a scheme
	scheme := runtime.NewScheme()

	// Define serializer options
	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		scheme,
		scheme, json.SerializerOptions{
			Yaml: true,
		},
	)

	var objects []*resource.Info
	for _, node := range manifest {
		// Get the GVK of the rNode
		gvk, err := getGVKForNode(node)
		if err != nil {
			return nil, fmt.Errorf("failed to get the GVK of rNode: %v", err)
		}

		// Add the GVK to scheme
		err = addSchemeForGVK(scheme, gvk)
		if err != nil {
			return nil, fmt.Errorf("failed to add GVK %q to scheme: %v", gvk, err)
		}

		// Convert the rNode to JSON bytes
		jsonData, err := node.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("error marshaling RNode to JSON: %w", err)
		}

		// Decode the JSON data into a Kubernetes runtime.Object
		obj, _, err := serializer.Decode(jsonData, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("error decoding JSON to runtime.Object: %w", err)
		}

		objects = append(objects, &resource.Info{Object: obj})
	}

	return objects, nil
}

// getGVKForNode returns GVK from an resource YAML node
func getGVKForNode(node *yaml.RNode) (schema.GroupVersionKind, error) {
	// Retrieve the apiVersion field from the RNode
	apiVersionNode, err := node.Pipe(yaml.Lookup(`apiVersion`))
	if err != nil || apiVersionNode == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion not found in RNode: %v", err)
	}

	// Retrieve the kind field from the RNode
	kindNode, err := node.Pipe(yaml.Lookup(`kind`))
	if err != nil || kindNode == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("kind not found in RNode: %v", err)
	}

	// Extract values
	apiVersion := apiVersionNode.YNode().Value
	kind := kindNode.YNode().Value

	// Parse the apiVersion to get GroupVersion
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("error parsing apiVersion: %v", err)
	}

	return gv.WithKind(kind), nil
}

// Mutex to protect concurrent access to the scheme
var schemeMutex sync.Mutex

// Registry to hold AddToScheme functions for each API group.
// Add more GroupVersion to AddToScheme func mappings if required by tests.
var addToSchemeRegistry = map[schema.GroupVersion]func(*runtime.Scheme) error{
	corev1.SchemeGroupVersion: corev1.AddToScheme,
	appsv1.SchemeGroupVersion: appsv1.AddToScheme,
}

// addSchemeForGVK dynamically adds GroupVersion to scheme
func addSchemeForGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) error {
	schemeMutex.Lock()
	defer schemeMutex.Unlock()

	// Exit early if GroupVersion is already registered
	gv := gvk.GroupVersion()
	if scheme.IsVersionRegistered(gv) {
		return nil
	}

	// Look up the function corresponding to current GroupVersion
	addToSchemeFunc, exists := addToSchemeRegistry[gv]
	if !exists {
		return fmt.Errorf("no AddToScheme function registered for %s", gv)
	}

	// Register the GroupVersion in the scheme
	if err := addToSchemeFunc(scheme); err != nil {
		return fmt.Errorf("failed to add scheme for %s: %w", gv, err)
	}

	return nil
}
