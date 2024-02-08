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

package kube // import "helm.sh/helm/v3/pkg/kube"

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestResourceList(t *testing.T) {
	t.Run("Different Pod mapping", func(t *testing.T) {
		mapping := &meta.RESTMapping{
			Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "pod"},
		}

		info := func(name string) *resource.Info {
			return &resource.Info{Name: name, Mapping: mapping}
		}

		var r1, r2 ResourceList
		r1 = []*resource.Info{info("foo"), info("bar")}
		r2 = []*resource.Info{info("bar")}

		if r1.Get(info("bar")).Mapping.Resource.Resource != "pod" {
			t.Error("expected get pod")
		}

		diff := r1.Difference(r2)
		if len(diff) != 1 {
			t.Error("expected 1 result")
		}

		if !diff.Contains(info("foo")) {
			t.Error("expected diff to return foo")
		}

		inter := r1.Intersect(r2)
		if len(inter) != 1 {
			t.Error("expected 1 result")
		}

		if !inter.Contains(info("bar")) {
			t.Error("expected intersect to return bar")
		}
	})

	t.Run("Different GroupVersionKind mapping", func(t *testing.T) {
		mappingCertManagerOld := &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "certmanager.k8s.io", Version: "v1alpha1", Resource: "certificate"},
			GroupVersionKind: schema.GroupVersionKind{Group: "certmanager.k8s.io", Version: "v1alpha1", Kind: "Certificate"},
		}
		mappingCertManager := &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificate"},
			GroupVersionKind: schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"},
		}

		info := func(name string, mapping *meta.RESTMapping) *resource.Info {
			return &resource.Info{Name: name, Mapping: mapping}
		}

		var r1, r2 ResourceList
		r1 = []*resource.Info{info("someCert", mappingCertManagerOld)}
		r2 = []*resource.Info{info("someCert", mappingCertManager)}

		diff := r1.Difference(r2)
		if len(diff) != 1 {
			t.Error("expected 1 result")
		}

		if diff.Contains(info("someCert", mappingCertManager)) {
			t.Error("expected diff to return old cert-manager GroupVersionKind")
		}

		if !diff.Contains(info("someCert", mappingCertManagerOld)) {
			t.Error("expected diff to not return new cert-manager GroupVersionKind")
		}

		inter := r1.Intersect(r2)
		if len(inter) != 0 {
			t.Error("expected 0 result")
		}
	})
}
