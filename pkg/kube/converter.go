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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
)

// NativeScheme is a clean *runtime.Scheme with _only_ Kubernetes
// native resources added to it. This is required to break free of custom resources
// that may have been added to scheme.Scheme due to Helm being used as a package in
// combination with e.g. a versioned kube client. If we would not do this, the client
// may attempt to perform e.g. a 3-way-merge strategy patch for custom resources.
// DO NOT MUTATE, this is a shared variable.
var NativeScheme *runtime.Scheme

func init() {
	NativeScheme = runtime.NewScheme()
	if err := scheme.AddToScheme(NativeScheme); err != nil {
		panic(err)
	}
	// API extensions are not in the above scheme set,
	// and must thus be added separately.
	if err := apiextensionsv1beta1.AddToScheme(NativeScheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(NativeScheme); err != nil {
		panic(err)
	}
}

// AsVersioned converts the given info into a runtime.Object with the correct
// group and version set
func AsVersioned(info *resource.Info) runtime.Object {
	return convertWithMapper(info.Object, info.Mapping)
}

// convertWithMapper converts the given object with the optional provided
// RESTMapping. If no mapping is provided, the default schema versioner is used
func convertWithMapper(obj runtime.Object, mapping *meta.RESTMapping) runtime.Object {
	var gv = runtime.GroupVersioner(schema.GroupVersions(NativeScheme.PrioritizedVersionsAllGroups()))
	if mapping != nil {
		gv = mapping.GroupVersionKind.GroupVersion()
	}
	if obj, err := NativeScheme.ConvertToVersion(obj, gv); err == nil {
		return obj
	}
	return obj
}
