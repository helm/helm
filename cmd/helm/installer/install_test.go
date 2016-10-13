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

	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestInstall(t *testing.T) {
	image := "gcr.io/kubernetes-helm/tiller:v2.0.0"

	fake := testclient.Fake{}
	fake.AddReactor("create", "deployments", func(action testclient.Action) (bool, runtime.Object, error) {
		obj := action.(testclient.CreateAction).GetObject().(*extensions.Deployment)
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

	err := Install(fake.Extensions(), "default", image, false, false)
	if err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}
}

func TestInstall_canary(t *testing.T) {
	fake := testclient.Fake{}
	fake.AddReactor("create", "deployments", func(action testclient.Action) (bool, runtime.Object, error) {
		obj := action.(testclient.CreateAction).GetObject().(*extensions.Deployment)
		i := obj.Spec.Template.Spec.Containers[0].Image
		if i != "gcr.io/kubernetes-helm/tiller:canary" {
			t.Errorf("expected canary image, got '%s'", i)
		}
		return true, obj, nil
	})

	err := Install(fake.Extensions(), "default", "", true, false)
	if err != nil {
		t.Errorf("unexpected error: %#+v", err)
	}
}
