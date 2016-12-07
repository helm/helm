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
	"reflect"
	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	testcore "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	"k8s.io/helm/pkg/version"
)

func TestDeploymentManifest(t *testing.T) {

	tests := []struct {
		name   string
		image  string
		canary bool
		expect string
	}{
		{"default", "", false, "gcr.io/kubernetes-helm/tiller:" + version.Version},
		{"canary", "example.com/tiller", true, "gcr.io/kubernetes-helm/tiller:canary"},
		{"custom", "example.com/tiller:latest", false, "example.com/tiller:latest"},
	}

	for _, tt := range tests {

		o, err := DeploymentManifest(api.NamespaceDefault, tt.image, tt.canary)
		if err != nil {
			t.Fatalf("%s: error %q", tt.name, err)
		}
		var dep extensions.Deployment
		if err := yaml.Unmarshal([]byte(o), &dep); err != nil {
			t.Fatalf("%s: error %q", tt.name, err)
		}

		if got := dep.Spec.Template.Spec.Containers[0].Image; got != tt.expect {
			t.Errorf("%s: expected image %q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestInstall(t *testing.T) {
	image := "gcr.io/kubernetes-helm/tiller:v2.0.0"

	fake := fake.NewSimpleClientset()
	fake.AddReactor("create", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		obj := action.(testcore.CreateAction).GetObject().(*extensions.Deployment)
		l := obj.GetLabels()
		if reflect.DeepEqual(l, map[string]string{"app": "helm"}) {
			t.Errorf("expected labels = '', got '%s'", l)
		}
		i := obj.Spec.Template.Spec.Containers[0].Image
		if i != image {
			t.Errorf("expected image = '%s', got '%s'", image, i)
		}
		return true, obj, nil
	})

	err := Install(fake.Extensions(), api.NamespaceDefault, image, false, false)
	if err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}
}

func TestInstall_canary(t *testing.T) {
	fake := fake.NewSimpleClientset()
	fake.AddReactor("create", "deployments", func(action testcore.Action) (bool, runtime.Object, error) {
		obj := action.(testcore.CreateAction).GetObject().(*extensions.Deployment)
		i := obj.Spec.Template.Spec.Containers[0].Image
		if i != "gcr.io/kubernetes-helm/tiller:canary" {
			t.Errorf("expected canary image, got '%s'", i)
		}
		return true, obj, nil
	})

	err := Install(fake.Extensions(), api.NamespaceDefault, "", true, false)
	if err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}
}
