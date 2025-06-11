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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestResourceList(t *testing.T) {
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
}

func TestIsMatchingInfo(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "group1", Version: "version1", Kind: "pod"}
	resourceInfo := resource.Info{Name: "name1", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvk}}

	gvkDiffGroup := schema.GroupVersionKind{Group: "diff", Version: "version1", Kind: "pod"}
	resourceInfoDiffGroup := resource.Info{Name: "name1", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvkDiffGroup}}
	if isMatchingInfo(&resourceInfo, &resourceInfoDiffGroup) {
		t.Error("expected resources not equal")
	}

	gvkDiffVersion := schema.GroupVersionKind{Group: "group1", Version: "diff", Kind: "pod"}
	resourceInfoDiffVersion := resource.Info{Name: "name1", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvkDiffVersion}}
	if isMatchingInfo(&resourceInfo, &resourceInfoDiffVersion) {
		t.Error("expected resources not equal")
	}

	gvkDiffKind := schema.GroupVersionKind{Group: "group1", Version: "version1", Kind: "deployment"}
	resourceInfoDiffKind := resource.Info{Name: "name1", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvkDiffKind}}
	if isMatchingInfo(&resourceInfo, &resourceInfoDiffKind) {
		t.Error("expected resources not equal")
	}

	resourceInfoDiffName := resource.Info{Name: "diff", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvk}}
	if isMatchingInfo(&resourceInfo, &resourceInfoDiffName) {
		t.Error("expected resources not equal")
	}

	resourceInfoDiffNamespace := resource.Info{Name: "name1", Namespace: "diff", Mapping: &meta.RESTMapping{GroupVersionKind: gvk}}
	if isMatchingInfo(&resourceInfo, &resourceInfoDiffNamespace) {
		t.Error("expected resources not equal")
	}

	gvkEqual := schema.GroupVersionKind{Group: "group1", Version: "version1", Kind: "pod"}
	resourceInfoEqual := resource.Info{Name: "name1", Namespace: "namespace1", Mapping: &meta.RESTMapping{GroupVersionKind: gvkEqual}}
	if !isMatchingInfo(&resourceInfo, &resourceInfoEqual) {
		t.Error("expected resources to be equal")
	}
}
