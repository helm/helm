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
	"time"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	testcore "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"k8s.io/helm/pkg/kube"
)

type fakeReaper struct {
	namespace string
	name      string
}

func (r *fakeReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *api.DeleteOptions) error {
	r.namespace = namespace
	r.name = name
	return nil
}

type fakeReaperFactory struct {
	cmdutil.Factory
	reaper kubectl.Reaper
}

func (f *fakeReaperFactory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	return f.reaper, nil
}

func TestUninstall(t *testing.T) {
	existingService := service(api.NamespaceDefault)
	existingDeployment := deployment(api.NamespaceDefault, "image", false)

	fc := &fake.Clientset{}
	fc.AddReactor("get", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, existingService, nil
	})
	fc.AddReactor("delete", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	fc.AddReactor("get", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, existingDeployment, nil
	})

	f, _, _, _ := cmdtesting.NewAPIFactory()
	r := &fakeReaper{}
	rf := &fakeReaperFactory{Factory: f, reaper: r}
	kc := &kube.Client{Factory: rf}

	if err := Uninstall(fc, kc, api.NamespaceDefault, false); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 3 {
		t.Errorf("unexpected actions: %v, expected 3 actions got %d", actions, len(actions))
	}

	if r.namespace != api.NamespaceDefault {
		t.Errorf("unexpected reaper namespace: %s", r.name)
	}

	if r.name != "tiller-deploy" {
		t.Errorf("unexpected reaper name: %s", r.name)
	}
}

func TestUninstall_serviceNotFound(t *testing.T) {
	existingDeployment := deployment(api.NamespaceDefault, "imageToReplace", false)

	fc := &fake.Clientset{}
	fc.AddReactor("get", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(api.Resource("services"), "1")
	})
	fc.AddReactor("get", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, existingDeployment, nil
	})

	f, _, _, _ := cmdtesting.NewAPIFactory()
	r := &fakeReaper{}
	rf := &fakeReaperFactory{Factory: f, reaper: r}
	kc := &kube.Client{Factory: rf}

	if err := Uninstall(fc, kc, api.NamespaceDefault, false); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 2 {
		t.Errorf("unexpected actions: %v, expected 2 actions got %d", actions, len(actions))
	}

	if r.namespace != api.NamespaceDefault {
		t.Errorf("unexpected reaper namespace: %s", r.name)
	}

	if r.name != "tiller-deploy" {
		t.Errorf("unexpected reaper name: %s", r.name)
	}
}

func TestUninstall_deploymentNotFound(t *testing.T) {
	existingService := service(api.NamespaceDefault)

	fc := &fake.Clientset{}
	fc.AddReactor("get", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, existingService, nil
	})
	fc.AddReactor("delete", "services", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	fc.AddReactor("get", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(api.Resource("deployments"), "1")
	})

	f, _, _, _ := cmdtesting.NewAPIFactory()
	r := &fakeReaper{}
	rf := &fakeReaperFactory{Factory: f, reaper: r}
	kc := &kube.Client{Factory: rf}

	if err := Uninstall(fc, kc, api.NamespaceDefault, false); err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}

	if actions := fc.Actions(); len(actions) != 3 {
		t.Errorf("unexpected actions: %v, expected 3 actions got %d", actions, len(actions))
	}

	if r.namespace != "" {
		t.Errorf("unexpected reaper namespace: %s", r.name)
	}

	if r.name != "" {
		t.Errorf("unexpected reaper name: %s", r.name)
	}
}
