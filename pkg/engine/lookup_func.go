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

package engine

import (
	"context"
	"log"
	"strings"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type lookupFunc = func(apiversion string, resource string, namespace string, name string) (map[string]interface{}, error)

// NewLookupFunction returns a function for looking up objects in the cluster.
//
// If the resource does not exist, no error is raised.
//
// This function is considered deprecated, and will be renamed in Helm 4. It will no
// longer be a public function.
func NewLookupFunction(config *rest.Config) lookupFunc {
	return func(apiversion string, resource string, namespace string, name string) (map[string]interface{}, error) {
		var client dynamic.ResourceInterface
		c, namespaced, err := getDynamicClientOnKind(apiversion, resource, config)
		if err != nil {
			return map[string]interface{}{}, err
		}
		if namespaced && namespace != "" {
			client = c.Namespace(namespace)
		} else {
			client = c
		}
		if name != "" {
			// this will return a single object
			obj, err := client.Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Just return an empty interface when the object was not found.
					// That way, users can use `if not (lookup ...)` in their templates.
					return map[string]interface{}{}, nil
				}
				return map[string]interface{}{}, err
			}
			return obj.UnstructuredContent(), nil
		}
		// this will return a list
		obj, err := client.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Just return an empty interface when the object was not found.
				// That way, users can use `if not (lookup ...)` in their templates.
				return map[string]interface{}{}, nil
			}
			return map[string]interface{}{}, err
		}
		return obj.UnstructuredContent(), nil
	}
}

// getDynamicClientOnUnstructured returns a dynamic client on an Unstructured type. This client can be further namespaced.
func getDynamicClientOnKind(apiversion string, kind string, config *rest.Config) (dynamic.NamespaceableResourceInterface, bool, error) {
	gvk := schema.FromAPIVersionAndKind(apiversion, kind)
	apiRes, err := getAPIResourceForGVK(gvk, config)
	if err != nil {
		log.Printf("[ERROR] unable to get apiresource from unstructured: %s , error %s", gvk.String(), err)
		return nil, false, errors.Wrapf(err, "unable to get apiresource from unstructured: %s", gvk.String())
	}
	gvr := schema.GroupVersionResource{
		Group:    apiRes.Group,
		Version:  apiRes.Version,
		Resource: apiRes.Name,
	}
	intf, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Printf("[ERROR] unable to get dynamic client %s", err)
		return nil, false, err
	}
	res := intf.Resource(gvr)
	return res, apiRes.Namespaced, nil
}

func getAPIResourceForGVK(gvk schema.GroupVersionKind, config *rest.Config) (metav1.APIResource, error) {
	res := metav1.APIResource{}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Printf("[ERROR] unable to create discovery client %s", err)
		return res, err
	}
	resList, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		log.Printf("[ERROR] unable to retrieve resource list for: %s , error: %s", gvk.GroupVersion().String(), err)
		return res, err
	}
	for _, resource := range resList.APIResources {
		// if a resource contains a "/" it's referencing a subresource. we don't support suberesource for now.
		if resource.Kind == gvk.Kind && !strings.Contains(resource.Name, "/") {
			res = resource
			res.Group = gvk.Group
			res.Version = gvk.Version
			break
		}
	}
	return res, nil
}
