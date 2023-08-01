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

package rules // import "helm.sh/helm/v3/pkg/lint/rules"

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/deprecation"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// This should be set in the Makefile based on the version of client-go being imported.
	// These constants will be overwritten with LDFLAGS. The version components must be
	// strings in order for LDFLAGS to set them.
	k8sVersionMajor = "1"
	k8sVersionMinor = "20"
)

// deprecatedAPIError indicates than an API is deprecated in Kubernetes
type deprecatedAPIError struct {
	Deprecated string
	Message    string
}

func (e deprecatedAPIError) Error() string {
	msg := e.Message
	return msg
}

func validateNoDeprecations(resource *K8sYamlStruct) error {
	// if `resource` does not have an APIVersion or Kind, we cannot test it for deprecation
	if resource.APIVersion == "" {
		return nil
	}
	if resource.Kind == "" {
		return nil
	}

	runtimeObject, err := resourceToRuntimeObject(resource)
	if err != nil {
		// do not error for non-kubernetes resources
		if runtime.IsNotRegisteredError(err) {
			return nil
		}
		return err
	}
	maj, err := strconv.Atoi(k8sVersionMajor)
	if err != nil {
		return err
	}
	min, err := strconv.Atoi(k8sVersionMinor)
	if err != nil {
		return err
	}

	if !deprecation.IsDeprecated(runtimeObject, maj, min) {
		return nil
	}
	gvk := fmt.Sprintf("%s %s", resource.APIVersion, resource.Kind)
	return deprecatedAPIError{
		Deprecated: gvk,
		Message:    deprecation.WarningMessage(runtimeObject),
	}
}

func resourceToRuntimeObject(resource *K8sYamlStruct) (runtime.Object, error) {
	scheme := runtime.NewScheme()
	kscheme.AddToScheme(scheme)

	gvk := schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind)
	out, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	out.GetObjectKind().SetGroupVersionKind(gvk)
	return out, nil
}
