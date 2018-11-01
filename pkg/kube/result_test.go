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

package kube // import "k8s.io/helm/pkg/kube"

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
)

func TestResult(t *testing.T) {
	mapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "pod"},
	}

	info := func(name string) *resource.Info {
		return &resource.Info{Name: name, Mapping: mapping}
	}

	var r1, r2 Result
	r1 = []*resource.Info{info("foo"), info("bar")}
	r2 = []*resource.Info{info("bar")}

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
