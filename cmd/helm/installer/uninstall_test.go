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

package installer // import "k8s.io/helm/cmd/helm/installer"

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testcore "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestUninstall(t *testing.T) {
	fc := &fake.Clientset{}
	opts := &Options{Namespace: api.NamespaceDefault}
	if err := Uninstall(fc, opts); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 7 {
		t.Errorf("unexpected actions: %v, expected 7 actions got %d", actions, len(actions))
	}
}

func TestUninstall_serviceNotFound(t *testing.T) {
	fc := &fake.Clientset{}
	fc.AddReactor("delete", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "1")
	})

	opts := &Options{Namespace: api.NamespaceDefault}
	if err := Uninstall(fc, opts); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 7 {
		t.Errorf("unexpected actions: %v, expected 7 actions got %d", actions, len(actions))
	}
}

func TestUninstall_deploymentNotFound(t *testing.T) {
	fc := &fake.Clientset{}
	fc.AddReactor("delete", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(api.Resource("deployments"), "1")
	})

	opts := &Options{Namespace: api.NamespaceDefault}
	if err := Uninstall(fc, opts); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 7 {
		t.Errorf("unexpected actions: %v, expected 7 actions got %d", actions, len(actions))
	}
}

func TestUninstall_secretNotFound(t *testing.T) {
	fc := &fake.Clientset{}
	fc.AddReactor("delete", "secrets", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(api.Resource("secrets"), "1")
	})

	opts := &Options{Namespace: api.NamespaceDefault}
	if err := Uninstall(fc, opts); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 7 {
		t.Errorf("unexpected actions: %v, expect 7 actions got %d", actions, len(actions))
	}
}
