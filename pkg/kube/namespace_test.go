/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestEnsureNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := ensureNamespace(client, "foo"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := ensureNamespace(client, "foo"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := client.Core().Namespaces().Get("foo", metav1.GetOptions{}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}
